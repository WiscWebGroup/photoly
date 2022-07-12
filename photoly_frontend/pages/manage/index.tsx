import {Avatar, Center, Divider, Heading, HStack, IconButton, Text, VStack} from "@chakra-ui/react";
import {AiOutlineEdit} from 'react-icons/ai'
import React from "react";
import useToken from "../../hooks/useToken";


const Manage: React.FC = () => {
    const token = useToken()

    return (
        <Center>
            <VStack shadow={"lg"} w={"50%"} rounded={"lg"} m={8} p={8}>
                <Heading>Profile</Heading>
                <HStack justifyContent={"space-evenly"} w={"100%"}>
                    <VStack>
                        <Avatar
                            borderRadius='full'
                            h={32}
                            w={32}
                            src={`user/getAvatar/${token}`}
                        />
                        <Text fontWeight={"semibold"} fontSize={"xl"}>Username</Text>
                        <IconButton variant={"ghost"} h={6} aria-label={"edit name"} icon={<AiOutlineEdit/>}/>

                    </VStack>

                    <Text>You joined on 2001/11/20</Text>
                </HStack>

                <Divider/>
                <Text fontSize={"2xl"}>Email</Text>
                <Text>123456789@qq.com</Text>
                <IconButton variant={"ghost"} h={6} aria-label={"edit name"} icon={<AiOutlineEdit/>}/>

                <Divider/>
                <Text fontSize={"2xl"}>API</Text>
            </VStack>
        </Center>
    )
}
export default Manage