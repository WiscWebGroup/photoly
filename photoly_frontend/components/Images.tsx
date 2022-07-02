import {Box, Button, Flex, Image, Text***REMOVED*** from "@chakra-ui/react";
import ImageContextMenu from "./ImageContextMenu";
import {useContextMenu***REMOVED*** from "../hooks/useContextMenu";
import React from "react";
import ImageView from "./ImageView";
import {AiOutlineFolderOpen***REMOVED*** from 'react-icons/ai'

const Images: React.FC = () => {
    const [showContext, handleContextMenu, anchorPoint] = useContextMenu()
    const [isModalOpen, setIsModalOpen] = React.useState(false)
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
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={handleContextMenu***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg' onClick={() => setIsModalOpen(true)***REMOVED***/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={handleContextMenu***REMOVED*** onClick={() => setIsModalOpen(true)***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg'/>
                </Box>
            </Flex>
   ***REMOVED*****REMOVED***showContext && <ImageContextMenu x={anchorPoint.x***REMOVED*** y={anchorPoint.y***REMOVED***/>***REMOVED***
            <ImageView isModalOpen={isModalOpen***REMOVED*** setIsModelOpen={setIsModalOpen***REMOVED***/>
        </>

    )
***REMOVED***
export default Images