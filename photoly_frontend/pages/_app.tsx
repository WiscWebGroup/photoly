import '../styles/globals.css'
import {ChakraProvider***REMOVED*** from '@chakra-ui/react'
import {AppProps***REMOVED*** from 'next/app'
import React from "react";
import Head from "next/head";
import {Provider***REMOVED*** from 'react-redux'
import store from '../redux/store'

function MyApp({ Component, pageProps ***REMOVED***: AppProps): React.ReactNode {
  return (
      <>
          <Head>
              <title>PHOTOLY</title>
          </Head>
          <ChakraProvider>
            <Provider store={store***REMOVED***>
              <Component {...pageProps***REMOVED*** />
            </Provider>
          </ChakraProvider>
      </>

  )
***REMOVED***

export default MyApp
