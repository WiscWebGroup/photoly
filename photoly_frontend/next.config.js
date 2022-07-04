/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: '/user/:path*',
        destination: 'http://localhost:8080/user/:path*'
      }
    ]
  }
}

module.exports = nextConfig
