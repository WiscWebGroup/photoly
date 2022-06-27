import {Button, Heading, HStack, Image, Avatar} from "@chakra-ui/react";

export default function Navbar() {
    return (
        <HStack bg={"teal.400"} w={"100%"} h={16} color={"white"} p={4}
                justifyContent={"space-between"}>
            <Heading ml={4}>PHOTOLY LOGO</Heading>
            <HStack h={"inherit"} p={2} spacing={4} mr={4}>
                <Button variant={"link"} color={"white"}>Logout</Button>
                <Button variant={"link"} color={"white"}>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"}
                    maxW={"100%"}
                    src='https://bit.ly/dan-abramov'
                    alt='Avatar'
                />
            </HStack>
        </HStack>
    )

}
