import {Box, Button, Flex, Image, Text} from "@chakra-ui/react";
import ImageContextMenu from "./ImageContextMenu";
import {useContextMenu} from "../hooks/useContextMenu";
import React from "react";
import ImageView from "./ImageView";
import {AiOutlineFolderOpen} from 'react-icons/ai'

const Images: React.FC = () => {
    const [showContext, handleContextMenu, anchorPoint] = useContextMenu()
    const [isModalOpen, setIsModalOpen] = React.useState(false)
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
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg' onClick={() => setIsModalOpen(true)}/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu} onClick={() => setIsModalOpen(true)}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
            </Flex>
            {showContext && <ImageContextMenu x={anchorPoint.x} y={anchorPoint.y}/>}
            <ImageView isModalOpen={isModalOpen} setIsModelOpen={setIsModalOpen}/>
        </>

    )
}
export default Images