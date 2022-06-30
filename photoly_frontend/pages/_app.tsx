import '../styles/globals.css'
import {ChakraProvider} from '@chakra-ui/react'
import {AppProps} from 'next/app'
import React from "react";

function MyApp({ Component, pageProps }: AppProps): React.ReactNode {
  return (
      <ChakraProvider>
              <Component {...pageProps} />
      </ChakraProvider>
  )
}

export default MyApp
