import '../styles/globals.css'
import {ChakraProvider***REMOVED*** from '@chakra-ui/react'

function MyApp({ Component, pageProps ***REMOVED***) {
  return (
      <ChakraProvider>
              <Component {...pageProps***REMOVED*** />
      </ChakraProvider>
  )
***REMOVED***

export default MyApp
