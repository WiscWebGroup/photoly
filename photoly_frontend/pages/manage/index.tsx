import {Avatar, Center, Heading, HStack, VStack, Text, IconButton, Divider***REMOVED*** from "@chakra-ui/react";
import {AiOutlineEdit***REMOVED*** from 'react-icons/ai'
import React from "react";

const Manage: React.FC = () => {

    return (
        <Center>
            <VStack shadow={"lg"***REMOVED*** w={"50%"***REMOVED*** rounded={"lg"***REMOVED*** m={8***REMOVED*** p={8***REMOVED***>
                <Heading>Profile</Heading>
                <HStack justifyContent={"space-evenly"***REMOVED*** w={"100%"***REMOVED***>
                    <VStack>
                        <Avatar
                            borderRadius='full'
                            h={32***REMOVED***
                            w={32***REMOVED***
                            src='https://bit.ly/dan-abramov'
                        />
                        <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Username</Text>
                        <IconButton variant={"ghost"***REMOVED*** h={6***REMOVED*** aria-label={"edit name"***REMOVED*** icon={<AiOutlineEdit/>***REMOVED***/>

                    </VStack>

                    <Text>You joined on 2001/11/20</Text>
                </HStack>

                <Divider/>
                <Text fontSize={"2xl"***REMOVED***>Email</Text>
                <Text>123456789@qq.com</Text>
                <IconButton variant={"ghost"***REMOVED*** h={6***REMOVED*** aria-label={"edit name"***REMOVED*** icon={<AiOutlineEdit/>***REMOVED***/>

                <Divider/>
                <Text fontSize={"2xl"***REMOVED***>API</Text>
            </VStack>
        </Center>
    )
***REMOVED***
export default Manage