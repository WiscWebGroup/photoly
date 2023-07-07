import {Button, Text, HStack, Avatar} from "@chakra-ui/react";
import {useRouter} from "next/router";
import Image from 'next/image'
import React from "react";
import useLocalStorage, {TOKEN_KEY} from "../hooks/useLocalStorage";
import useToken from "../hooks/useToken";

const Navbar: React.FC = () => {
    const router = useRouter()
    const {removeLS} = useLocalStorage(TOKEN_KEY)
    const token = useToken()
    const handleLogout = () => {
        router.push("/login")
        removeLS()
    }
    return (
        <HStack bg={"teal.400"} w={"100%"} h={16} color={"white"} p={4}
                justifyContent={"space-between"}>
            <HStack  cursor={"pointer"} onClick={() => router.push("/home")}>
                <Image src={"/favicon.ico"} alt={"logo"} width={"50px"} height={"50px"} data-testid={"logo"}/>
                <Text fontSize={"xl"} fontWeight={"medium"}>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"} p={2} spacing={4} mr={4}>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/home")}>Home</Button>
                <Button variant={"link"} color={"white"} onClick={handleLogout}>Logout</Button>
                <Button variant={"link"} color={"white"} onClick={() => router.push("/manage")}>Manage</Button>
                <Avatar
                    borderRadius='full'
                    maxH={"100%"}
                    maxW={"100%"}
                    src={`/user/getAvatar/${token}`}
                    data-testid={"avatar"}
                    onClick={() => router.push("/manage")}
                />
            </HStack>
        </HStack>
    )
}
export default Navbar
