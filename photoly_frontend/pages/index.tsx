import {Button, Center} from "@chakra-ui/react";
import Link from "next/link";
import React from "react";

const Home:React.FC = () => {
  return (
      <Center h={"100vh"}>
          <Link href="/login">
              <Button size="lg" variant={"link"} fontSize={"5xl"}>Login Here!</Button>
          </Link>
      </Center>

  )
}

export default Home
