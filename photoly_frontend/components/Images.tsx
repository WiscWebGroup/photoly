import {Button, Flex, Text***REMOVED*** from "@chakra-ui/react";
import React from "react";
import {AiOutlineFolderOpen***REMOVED*** from 'react-icons/ai'
import ImageItem from "./ImageItem";

const Images: React.FC = () => {
    return (
        <>
            <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Folders</Text>
            <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' ml={2***REMOVED***>
                    Folder1
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' ml={2***REMOVED***>
                    Folder2
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' ml={2***REMOVED***>
                    Folder3
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' ml={2***REMOVED***>
                    Folder4
                </Button>
            </Flex>
            <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Images</Text>
            <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
            </Flex>
        </>

    )
***REMOVED***
export default Images