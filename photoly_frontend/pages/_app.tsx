import '../styles/globals.css'
import {ChakraProvider} from '@chakra-ui/react'
import {AppProps} from 'next/app'
import React from "react";
import Head from "next/head";
import {Provider} from 'react-redux'
import store from '../redux/store'

function MyApp({ Component, pageProps }: AppProps): React.ReactNode {
  return (
      <>
          <Head>
              <title>PHOTOLY</title>
          </Head>
          <ChakraProvider>
            <Provider store={store}>
              <Component {...pageProps} />
            </Provider>
          </ChakraProvider>
      </>

  )
}

export default MyApp
