import {Avatar, Center, Divider, Heading, HStack, IconButton, Text, useDisclosure, VStack} from "@chakra-ui/react";
import {AiOutlineEdit} from 'react-icons/ai'
import React, {useEffect, useState} from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../../components/ChangeInfoDrawer";

interface userInfo {
    userId: number,
    userName: string,
    email: string,
    createDate: string,
    role: string,
    uuid: string
}

const Manage: React.FC = () => {
    const token = useToken()
    const {get} = useApi()
    const [info, setInfo] = useState<userInfo>()
    const {isOpen, onOpen, onClose} = useDisclosure()

    const getInfo = async () => {
        const response = await get("/user/getInfo", {headers: {"HRD-token": token}})
        if (!!response && response.data.msgCode === 200) {
            setInfo(response.data.t)
        }
    }
    useEffect(() => {
        if (!!token) {
            getInfo()
        }
    }, [token])


    return (
        <>
            <ChangeInfoDrawer isOpen={isOpen} onClose={onClose}/>
            <Center>
                <VStack shadow={"lg"} w={"50%"} rounded={"lg"} m={8} p={8}>
                    <Heading>Profile</Heading>
                    <IconButton variant={"ghost"} aria-label={"Edit profile"} icon={<AiOutlineEdit/>}
                                onClick={onOpen}/>
                    <HStack justifyContent={"space-evenly"} w={"100%"}>
                        <VStack>
                            <Avatar
                                borderRadius='full'
                                h={32}
                                w={32}
                                src={`user/getAvatar/${token}`}
                            />
                            <Text fontWeight={"semibold"} fontSize={"xl"}>{info?.userName}</Text>

                        </VStack>

                        <Text>You joined on {info?.createDate.substr(0, 10)}</Text>
                    </HStack>

                    <Divider/>
                    <Text fontSize={"2xl"}>Email</Text>
                    <Text>{info?.email}</Text>

                    <Divider/>
                    <Text fontSize={"2xl"}>API</Text>
                </VStack>
            </Center>
        </>

    )
}
export default Manage