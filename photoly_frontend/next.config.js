***REMOVED*** @type {import('next').NextConfig***REMOVED******REMOVED***
const serverAddr = 'http://129.146.3.179:8088'
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
  ***REMOVED***
        source: '/user/:path*',
        destination: `${serverAddr***REMOVED***/user/:path*`
  ***REMOVED***,
  ***REMOVED***
          source: '/cred/:path*',
          destination: `${serverAddr***REMOVED***/cred/:path*`
  ***REMOVED***,
  ***REMOVED***
          source: '/photo/:path*',
          destination: `${serverAddr***REMOVED***/photo/:path*`
  ***REMOVED***,
  ***REMOVED***
          source: '/namespace/:path*',
          destination: `${serverAddr***REMOVED***/namespace/:path*`
  ***REMOVED***,
  ***REMOVED***
          source: '/gallery/:path*',
          destination: `${serverAddr***REMOVED***/gallery/:path*`
  ***REMOVED***,
  ***REMOVED***
          source: '/tag/:path*',
          destination: `${serverAddr***REMOVED***/tag/:path*`
  ***REMOVED***
    ]
  ***REMOVED***
***REMOVED***

module.exports = nextConfig
