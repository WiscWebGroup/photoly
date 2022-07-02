import {Button, Center***REMOVED*** from "@chakra-ui/react";
import Link from "next/link";
import React from "react";

const Home:React.FC = () => {
  return (
      <>
          <Center h={"100vh"***REMOVED***>
              <Link href="/login">
                  <Button size="lg" variant={"link"***REMOVED*** fontSize={"5xl"***REMOVED***>Login Here!</Button>
              </Link>
          </Center>
      </>


  )
***REMOVED***

export default Home
