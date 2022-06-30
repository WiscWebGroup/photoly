import '../styles/globals.css'
import {ChakraProvider***REMOVED*** from '@chakra-ui/react'
import {AppProps***REMOVED*** from 'next/app'
import React from "react";

function MyApp({ Component, pageProps ***REMOVED***: AppProps): React.ReactNode {
  return (
      <ChakraProvider>
              <Component {...pageProps***REMOVED*** />
      </ChakraProvider>
  )
***REMOVED***

export default MyApp
