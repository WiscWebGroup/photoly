import {Button, Text, HStack, Avatar***REMOVED*** from "@chakra-ui/react";
import {useRouter***REMOVED*** from "next/router";
import Image from 'next/image'
import React from "react";

const Navbar: React.FC = () => {
    const router = useRouter()
    return (
        <HStack bg={"teal.400"***REMOVED*** w={"100%"***REMOVED*** h={16***REMOVED*** color={"white"***REMOVED*** p={4***REMOVED***
                justifyContent={"space-between"***REMOVED***>
            <HStack>
                <Image src={"/logo.png"***REMOVED*** alt={"logo"***REMOVED*** width={"50px"***REMOVED*** height={"50px"***REMOVED*** data-testid={"logo"***REMOVED***/>
                <Text fontSize={"xl"***REMOVED*** fontWeight={"medium"***REMOVED***>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"***REMOVED*** p={2***REMOVED*** spacing={4***REMOVED*** mr={4***REMOVED***>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED*** onClick={() => router.push("/login")***REMOVED***>Logout</Button>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED*** onClick={() => router.push("/manage")***REMOVED***>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"***REMOVED***
                    maxW={"100%"***REMOVED***
                    src='https://bit.ly/dan-abramov'
                    data-testid={"avatar"***REMOVED***
                />
            </HStack>
        </HStack>
    )
***REMOVED***
export default Navbar
