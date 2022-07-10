***REMOVED*** @type {import('next').NextConfig***REMOVED******REMOVED***
const serverAddr = 'http://129.146.3.179:8088/'
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
  ***REMOVED***
        source: '/user/:path*',
        destination: `${serverAddr***REMOVED***user/:path*`
  ***REMOVED***
    ]
  ***REMOVED***
***REMOVED***

module.exports = nextConfig
