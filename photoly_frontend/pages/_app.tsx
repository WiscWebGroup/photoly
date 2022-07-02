import '../styles/globals.css'
import {ChakraProvider} from '@chakra-ui/react'
import {AppProps} from 'next/app'
import React from "react";
import Head from "next/head";

function MyApp({ Component, pageProps }: AppProps): React.ReactNode {
  return (
      <>
          <Head>
              <title>PHOTOLY</title>
          </Head>
          <ChakraProvider>
              <Component {...pageProps} />
          </ChakraProvider>
      </>

  )
}

export default MyApp
