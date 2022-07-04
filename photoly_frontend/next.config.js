***REMOVED*** @type {import('next').NextConfig***REMOVED******REMOVED***
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
  ***REMOVED***
        source: '/user/:path*',
        destination: 'http://localhost:8080/user/:path*'
  ***REMOVED***
    ]
  ***REMOVED***
***REMOVED***

module.exports = nextConfig
