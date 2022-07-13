/** @type {import('next').NextConfig} */
const serverAddr = 'http://129.146.3.179:8088'
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: '/user/:path*',
        destination: `${serverAddr}/user/:path*`
      }
    ]
  }
}

module.exports = nextConfig
