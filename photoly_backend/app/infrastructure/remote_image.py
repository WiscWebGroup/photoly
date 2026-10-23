"""Bounded public-image fetches with DNS validation and IP-pinned TLS connections."""
import ipaddress
import socket
import time
from urllib.parse import urlsplit, urljoin
import urllib3
from app.services.base import BusinessError


def public_target(url):
    if not isinstance(url, str) or len(url) > 8192 or any(ord(c) < 33 for c in url) or '\\' in url:
        raise BusinessError('invalid_image_url')
    try:
        parsed = urlsplit(url)
        if parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username is not None or parsed.password is not None:
            raise ValueError()
        port = parsed.port or (443 if parsed.scheme == 'https' else 80)
        host = parsed.hostname.encode('idna').decode('ascii')
        addresses = list(dict.fromkeys(row[4][0] for row in socket.getaddrinfo(host, port, type=socket.SOCK_STREAM)))
        if not addresses:
            raise ValueError()
        for address in addresses:
            ip = ipaddress.ip_address(address)
            if not ip.is_global or ip.is_multicast or ip.is_unspecified or '%' in address:
                raise ValueError()
            if isinstance(ip, ipaddress.IPv6Address) and (ip.ipv4_mapped or ip.sixtofour or ip.teredo):
                raise ValueError()
    except (ValueError, UnicodeError, OSError) as error:
        raise BusinessError('non_public_image_url') from error
    return parsed, host, port, addresses[0]


def download_image(url, destination, config):
    deadline = time.monotonic() + config['ONLINE_DOWNLOAD_TIMEOUT']
    limit = config['ONLINE_DOWNLOAD_MAX_BYTES']
    try:
        for redirect in range(4):
            parsed, host, port, address = public_target(url)
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise BusinessError('download_timeout', 408)
            kwargs = {'timeout': urllib3.Timeout(connect=min(config['ONLINE_DOWNLOAD_CONNECT_TIMEOUT'], remaining), read=min(remaining, 5)), 'retries': False}
            # No PoolManager, environment proxy or subsequent DNS lookup is involved.
            pool = (urllib3.HTTPSConnectionPool(address, port, server_hostname=host, assert_hostname=host, cert_reqs='CERT_REQUIRED', **kwargs)
                    if parsed.scheme == 'https' else urllib3.HTTPConnectionPool(address, port, **kwargs))
            response = None
            try:
                authority = ('[' + host + ']' if ':' in host else host) + (f':{port}' if port != (443 if parsed.scheme == 'https' else 80) else '')
                target = (parsed.path or '/') + ('?' + parsed.query if parsed.query else '')
                response = pool.urlopen('GET', target, headers={'Host': authority, 'Accept': 'image/*', 'Accept-Encoding': 'identity'}, redirect=False, preload_content=False)
                if response.status in (301, 302, 303, 307, 308):
                    if redirect == 3 or not response.headers.get('Location'):
                        raise BusinessError('too_many_redirects')
                    url = urljoin(url, response.headers['Location'])
                    continue
                if response.status != 200:
                    raise BusinessError('download_http_error')
                length = response.headers.get('Content-Length')
                if length and (not length.isdigit() or int(length) > limit):
                    raise BusinessError('download_too_large', 413)
                if response.headers.get('Content-Encoding', 'identity').lower() != 'identity':
                    raise BusinessError('unsupported_content_encoding')
                size = 0
                with destination.open('wb') as output:
                    while True:
                        if time.monotonic() >= deadline:
                            raise BusinessError('download_timeout', 408)
                        # read1 returns available bytes, so a drip-fed body cannot reset the deadline indefinitely.
                        chunk = response.read1(64 * 1024, decode_content=False)
                        if not chunk:
                            break
                        size += len(chunk)
                        if size > limit:
                            raise BusinessError('download_too_large', 413)
                        import shutil
                        if shutil.disk_usage(destination.parent).free < len(chunk) + config['UPLOAD_DISK_RESERVE_BYTES']:
                            raise BusinessError('insufficient_disk_space', 507)
                        output.write(chunk)
                if time.monotonic() >= deadline:
                    raise BusinessError('download_timeout', 408)
                return destination
            finally:
                if response:
                    response.close()
                pool.close()
    except urllib3.exceptions.TimeoutError as error:
        raise BusinessError('download_timeout', 408) from error
    except urllib3.exceptions.HTTPError as error:
        raise BusinessError('download_failed') from error


def download_bounded(url, destination, config):
    """A process deadline also bounds DNS resolution and a peer stalling response headers."""
    import json
    import subprocess
    import sys
    keys = ('ONLINE_DOWNLOAD_TIMEOUT', 'ONLINE_DOWNLOAD_CONNECT_TIMEOUT', 'ONLINE_DOWNLOAD_MAX_BYTES', 'UPLOAD_DISK_RESERVE_BYTES')
    try:
        result = subprocess.run([sys.executable, '-m', 'app.infrastructure.remote_image', str(destination)],
            input=json.dumps({'url': url, 'config': {key: config[key] for key in keys}}).encode(),
            capture_output=True, timeout=config['ONLINE_DOWNLOAD_TIMEOUT'])
    except subprocess.TimeoutExpired as error:
        raise BusinessError('download_timeout', 408) from error
    if result.returncode:
        reason = result.stdout.decode(errors='replace').strip()
        allowed = {'invalid_image_url', 'non_public_image_url', 'download_timeout', 'download_too_large',
                   'too_many_redirects', 'download_http_error', 'unsupported_content_encoding', 'insufficient_disk_space'}
        raise BusinessError(reason if reason in allowed else 'download_failed')


if __name__ == '__main__':
    import json
    import sys
    from pathlib import Path
    data = json.load(sys.stdin)
    try:
        download_image(data['url'], Path(sys.argv[1]), data['config'])
    except BusinessError as error:
        print(error.value)
        sys.exit(1)
