import React from "react";
import {
    Divider,
    Flex,
    HStack,
    Image,
    Tag,
    TagLabel,
    Text,
    useOutsideClick,
    VStack,
    Modal,
    ModalOverlay, ModalContent, ModalCloseButton
***REMOVED*** from "@chakra-ui/react";

interface ImageViewProps {
    isViewOpen: boolean,
    onViewClose: () => void
***REMOVED***

const ImageView: React.FC<ImageViewProps> = ({isViewOpen, onViewClose***REMOVED***) => {
    const ref = React.useRef() as React.MutableRefObject<HTMLDivElement>
    useOutsideClick({
        ref: ref,
        handler: onViewClose,
***REMOVED***)
    const tags = ["Animals", "Cute", "Furry", "White hair", "Landscape"]
    return (
        <>
            <Modal isOpen={isViewOpen***REMOVED*** onClose={onViewClose***REMOVED***>
                <ModalOverlay/>
                <ModalContent maxW={"85vw"***REMOVED***>
                    <ModalCloseButton/>
                    <HStack alignItems={"flex-start"***REMOVED***>
                        <Image src={"https://www.industrialempathy.com/img/remote/ZiClJf-1920w.jpg"***REMOVED*** alt={"image"***REMOVED***
                               rounded={"md"***REMOVED***
                               w={"80%"***REMOVED*** h={"100%"***REMOVED*** objectFit={"fill"***REMOVED***/>
                        <VStack w={"100%"***REMOVED*** h={"100%"***REMOVED*** p={4***REMOVED***>
                            <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Tags</Text>
                            <Flex flexWrap={"wrap"***REMOVED***>
             ***REMOVED*****REMOVED*****REMOVED*****REMOVED***tags.map((tag) =>
                                    <Tag
                                        size={"md"***REMOVED***
                                        key={tag***REMOVED***
                                        borderRadius='full'
                                        variant='solid'
                                        colorScheme='teal'
                                        m={1***REMOVED***
                                    >
                                        <TagLabel>{tag***REMOVED***</TagLabel>
                                    </Tag>
                                )***REMOVED***
                            </Flex>
                            <Divider/>
                            <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Albums</Text>
                            <Flex flexWrap={"wrap"***REMOVED***>
             ***REMOVED*****REMOVED*****REMOVED*****REMOVED***tags.map((tag) =>
                                    <Tag
                                        size={"md"***REMOVED***
                                        key={tag***REMOVED***
                                        borderRadius='full'
                                        variant='solid'
                                        colorScheme='gray'
                                        m={1***REMOVED***
                                    >
                                        <TagLabel>{tag***REMOVED***</TagLabel>
                                    </Tag>
                                )***REMOVED***
                            </Flex>
                            <Divider/>
                            <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Metadata</Text>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Name: </Text>
                                <Text>Sunset</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Date: </Text>
                                <Text>6/30/2002</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Path: </Text>
                                <Text>/beach</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Format: </Text>
                                <Text>png</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Shared: </Text>
                                <Text>No</Text>
                            </HStack>

                        </VStack>
                    </HStack>
                </ModalContent>
            </Modal>
        </>
    )
***REMOVED***

export default ImageView