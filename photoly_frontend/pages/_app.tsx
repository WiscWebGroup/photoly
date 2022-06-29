import '../styles/globals.css'
import {ChakraProvider***REMOVED*** from '@chakra-ui/react'
import {AppProps***REMOVED*** from 'next/app'

function MyApp({ Component, pageProps ***REMOVED***: AppProps) {
  return (
      <ChakraProvider>
              <Component {...pageProps***REMOVED*** />
      </ChakraProvider>
  )
***REMOVED***

export default MyApp
