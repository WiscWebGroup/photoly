import {Button, Heading, HStack, Image, Avatar***REMOVED*** from "@chakra-ui/react";

export default function Navbar() {
    return (
        <HStack bg={"teal.400"***REMOVED*** w={"100%"***REMOVED*** h={16***REMOVED*** color={"white"***REMOVED*** p={4***REMOVED***
                justifyContent={"space-between"***REMOVED***>
            <Heading ml={4***REMOVED***>PHOTOLY LOGO</Heading>
            <HStack h={"inherit"***REMOVED*** p={2***REMOVED*** spacing={4***REMOVED*** mr={4***REMOVED***>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED***>Logout</Button>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED***>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"***REMOVED***
                    maxW={"100%"***REMOVED***
                    src='https://bit.ly/dan-abramov'
                    alt='Avatar'
                />
            </HStack>
        </HStack>
    )

***REMOVED***
