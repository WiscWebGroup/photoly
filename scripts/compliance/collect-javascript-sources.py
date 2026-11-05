#!/usr/bin/env python3
"""Archive published packages AND pinned upstream sources for emitted browser/worker code.
Node-only tools are inventoried separately; they are not bundled into the web image.
"""
import base64,hashlib,io,json,pathlib,re,subprocess,tarfile,urllib.parse,urllib.request,time
root=pathlib.Path('/usr/share/photoly/licenses/frontend')
if not root.exists():raise SystemExit(0)
out=pathlib.Path('/compliance-sources/javascript');out.mkdir(parents=True,exist_ok=True)
subprocess.run(['apt-get','install','-y','--no-install-recommends','git'],check=True)
def fetch(url):
 for attempt in range(3):
  try:
   with urllib.request.urlopen(urllib.request.Request(url,headers={'User-Agent':'Photoly-source-archive'}),timeout=120) as r:return r.read()
  except Exception:
   if attempt==2:raise
   time.sleep(attempt+1)
keys=set()
for p in [root/'bundled-dependencies.json',*root.glob('worker-*-dependencies.json')]:
 keys.update(json.loads(p.read_text())['packages'])
records={p['name']+'@'+p['version']:p for p in json.loads((root/'dependencies.json').read_text())['packages']}
refs_cache={};archives={};manifest=[]
for key in sorted(keys):
 print('JavaScript source: '+key,flush=True)
 p=records[key];name=p['name'];version=p['version'];meta=json.loads(fetch('https://registry.npmjs.org/'+urllib.parse.quote(name,safe='')+'/'+version))
 published=fetch(meta['dist']['tarball']);integrity=meta['dist']['integrity'].split()[0];algorithm,want=integrity.split('-',1)
 if base64.b64encode(hashlib.new(algorithm,published).digest()).decode()!=want:raise RuntimeError('npm integrity mismatch: '+key)
 stem=re.sub(r'[^a-zA-Z0-9_.-]','_',key)
 (out/(stem+'.npm.tgz')).write_bytes(published)
 with tarfile.open(fileobj=io.BytesIO(published),mode='r:gz') as tar:
  package_file=tar.extractfile('package/package.json');package=json.load(package_file)
  if package['name']!=name or package['version']!=version:raise RuntimeError('wrong package archive: '+key)
  commit=meta.get('gitHead')
  if 'package/__commit.txt' in tar.getnames():commit=tar.extractfile('package/__commit.txt').read().decode().strip()
 repo=meta.get('repository',p['source']);repo=repo.get('url','') if isinstance(repo,dict) else repo
 repo=repo.split('github.com/')[-1].split('github.com:')[-1].split('/tree/')[0].split('#')[0].removesuffix('.git')
 if not re.fullmatch(r'[\w.-]+/[\w.-]+',repo):raise RuntimeError('source repository requires review: '+key+' '+repo)
 if repo not in refs_cache:
  lines=subprocess.check_output(['git','ls-remote','--tags','https://github.com/'+repo+'.git'],text=True).splitlines();refs_cache[repo]={line.split()[1]:line.split()[0] for line in lines}
 refs=refs_cache[repo]
 for tag,h in refs.items():
  if (h==commit and not tag.endswith('^{}')) or tag in ['refs/tags/v'+version,'refs/tags/'+version,'refs/tags/'+name+'@'+version]:
   commit=refs.get(tag+'^{}',h);break
 if not commit or not re.fullmatch('[a-f0-9]{7,40}',commit):raise RuntimeError('missing exact upstream revision: '+key)
 source='https://codeload.github.com/'+repo+'/tar.gz/'+commit
 if source not in archives:
  content=fetch(source);filename=repo.replace('/','_')+'-'+commit+'.tar.gz';(out/filename).write_bytes(content)
  archives[source]={'file':filename,'sha256':hashlib.sha256(content).hexdigest()}
 manifest.append({'package':key,'npm':{'file':stem+'.npm.tgz','url':meta['dist']['tarball'],'integrity':integrity,'sha256':hashlib.sha256(published).hexdigest()},'upstream':{'url':source,'revision':commit,**archives[source]}})
(out/'sources.json').write_text(json.dumps(manifest,indent=2)+'\n')
(out/'COMPLETE').write_text('Published npm archives and pinned upstream source archives collected for every emitted package.\n')
