import {Button, Flex, Text} from "@chakra-ui/react";
import React from "react";
import {AiOutlineFolderOpen} from 'react-icons/ai'
import ImageItem from "./ImageItem";

const Images: React.FC = () => {
    return (
        <>
            <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Folders</Text>
            <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' ml={2}>
                    Folder1
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' ml={2}>
                    Folder2
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' ml={2}>
                    Folder3
                </Button>
                <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' ml={2}>
                    Folder4
                </Button>
            </Flex>
            <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Images</Text>
            <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
            </Flex>
        </>

    )
}
export default Images