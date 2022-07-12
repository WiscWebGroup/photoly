import {Button, Text, HStack, Avatar} from "@chakra-ui/react";
import {useRouter} from "next/router";
import Image from 'next/image'
import React from "react";
import useLocalStorage, {TOKEN_KEY} from "../hooks/useLocalStorage";

const Navbar: React.FC = () => {
    const router = useRouter()
    const {remove} = useLocalStorage(TOKEN_KEY)
    const handleLogout = () => {
        router.push("/login")
        remove()
    }
    return (
        <HStack bg={"teal.400"} w={"100%"} h={16} color={"white"} p={4}
                justifyContent={"space-between"}>
            <HStack>
                <Image src={"/favicon.ico"} alt={"logo"} width={"50px"} height={"50px"} data-testid={"logo"}/>
                <Text fontSize={"xl"} fontWeight={"medium"}>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"} p={2} spacing={4} mr={4}>
                <Button variant={"link"} color={"white"} onClick={handleLogout}>Logout</Button>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/manage")}>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"}
                    maxW={"100%"}
                    src='https://bit.ly/dan-abramov'
                    data-testid={"avatar"}
                />
            </HStack>
        </HStack>
    )
}
export default Navbar
