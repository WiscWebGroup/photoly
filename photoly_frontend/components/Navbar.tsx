import {Button, Text, HStack, Avatar} from "@chakra-ui/react";
import {useRouter} from "next/router";
import Image from 'next/image'
import React from "react";

const Navbar: React.FC = () => {
    const router = useRouter()
    return (
        <HStack bg={"teal.400"} w={"100%"} h={16} color={"white"} p={4}
                justifyContent={"space-between"}>
            <HStack>
                <Image src={"/logo.png"} alt={"logo"} width={"50px"} height={"50px"}/>
                <Text fontSize={"xl"} fontWeight={"medium"}>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"} p={2} spacing={4} mr={4}>
                <Button variant={"link"} color={"white"}>Logout</Button>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/manage")}>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"}
                    maxW={"100%"}
                    src='https://bit.ly/dan-abramov'
                />
            </HStack>
        </HStack>
    )
}
export default Navbar
