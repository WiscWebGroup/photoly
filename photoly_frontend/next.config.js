/** @type {import('next').NextConfig} */
const serverAddr = 'http://129.146.3.179:8088'
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: '/user/:path*',
        destination: `${serverAddr}/user/:path*`
      },
      {
          source: '/cred/:path*',
          destination: `${serverAddr}/cred/:path*`
      },
      {
          source: '/photo/:path*',
          destination: `${serverAddr}/photo/:path*`
      },
      {
          source: '/namespace/:path*',
          destination: `${serverAddr}/namespace/:path*`
      },
      {
          source: '/gallery/:path*',
          destination: `${serverAddr}/gallery/:path*`
      },
      {
          source: '/tag/:path*',
          destination: `${serverAddr}/tag/:path*`
      }
    ]
  }
}

module.exports = nextConfig
