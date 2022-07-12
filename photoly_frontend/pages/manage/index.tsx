import {Avatar, Center, Divider, Heading, HStack, IconButton, Text, useDisclosure, VStack***REMOVED*** from "@chakra-ui/react";
import {AiOutlineEdit***REMOVED*** from 'react-icons/ai'
import React, {useEffect, useState***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import NameAndAvatarDrawer from "../../components/NameAndAvatarDrawer";

interface userInfo {
    userId: number,
    userName: string,
    email: string,
    createDate: string,
    role: string,
    uuid: string
***REMOVED***

const Manage: React.FC = () => {
    const token = useToken()
    const {get***REMOVED*** = useApi()
    const [info, setInfo] = useState<userInfo>()
    const {isOpen, onOpen, onClose***REMOVED*** = useDisclosure()

    const getInfo = async () => {
        const response = await get("/user/getInfo", {headers: {"HRD-token": token***REMOVED******REMOVED***)
        if (!!response && response.data.msgCode === 200) {
            setInfo(response.data.t)
***REMOVED***
***REMOVED***
    useEffect(() => {
        if (!!token) {
            getInfo()
***REMOVED***
***REMOVED***, [token])


    return (
        <>
            <NameAndAvatarDrawer isOpen={isOpen***REMOVED*** onClose={onClose***REMOVED***/>
            <Center>
                <VStack shadow={"lg"***REMOVED*** w={"50%"***REMOVED*** rounded={"lg"***REMOVED*** m={8***REMOVED*** p={8***REMOVED***>
                    <Heading>Profile</Heading>
                    <HStack justifyContent={"space-evenly"***REMOVED*** w={"100%"***REMOVED***>
                        <VStack>
                            <Avatar
                                borderRadius='full'
                                h={32***REMOVED***
                                w={32***REMOVED***
                                src={`user/getAvatar/${token***REMOVED***`***REMOVED***
                            />
                            <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>{info?.userName***REMOVED***</Text>
                            <IconButton variant={"ghost"***REMOVED*** h={6***REMOVED*** aria-label={"edit name"***REMOVED*** icon={<AiOutlineEdit/>***REMOVED***
                                        onClick={onOpen***REMOVED***/>

                        </VStack>

                        <Text>You joined on {info?.createDate.substr(0, 10)***REMOVED***</Text>
                    </HStack>

                    <Divider/>
                    <Text fontSize={"2xl"***REMOVED***>Email</Text>
                    <Text>{info?.email***REMOVED***</Text>
                    <IconButton variant={"ghost"***REMOVED*** h={6***REMOVED*** aria-label={"edit name"***REMOVED*** icon={<AiOutlineEdit/>***REMOVED***/>

                    <Divider/>
                    <Text fontSize={"2xl"***REMOVED***>API</Text>
                </VStack>
            </Center>
        </>

    )
***REMOVED***
export default Manage