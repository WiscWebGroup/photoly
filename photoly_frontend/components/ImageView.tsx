import React from "react";
import {
    Divider,
    Flex,
    HStack,
    Image,
    Modal,
    ModalCloseButton,
    ModalContent,
    ModalOverlay,
    Tag,
    TagLabel,
    Text,
    useOutsideClick,
    VStack
***REMOVED*** from "@chakra-ui/react";

interface ImageViewProps {
    isViewOpen: boolean,
    onViewClose: () => void,
    path: string | undefined,
    pname: string | undefined,
    date: string | undefined,
    orgsrc: string,
    tags: string[],
    albums: string[]
***REMOVED***

const ImageView: React.FC<ImageViewProps> = ({isViewOpen, onViewClose, path, pname, date, orgsrc, tags, albums***REMOVED***) => {
    const ref = React.useRef() as React.MutableRefObject<HTMLDivElement>
    useOutsideClick({
        ref: ref,
        handler: onViewClose,
***REMOVED***)
    return (
        <>
            <Modal isOpen={isViewOpen***REMOVED*** onClose={onViewClose***REMOVED***>
                <ModalOverlay/>
                <ModalContent maxW={"85vw"***REMOVED***>
                    <ModalCloseButton/>
                    <HStack alignItems={"flex-start"***REMOVED***>
                        <Image src={orgsrc***REMOVED*** alt={"image"***REMOVED***
                               rounded={"md"***REMOVED***
                               w={"80%"***REMOVED*** h={"100%"***REMOVED*** objectFit={"fill"***REMOVED***/>
                        <VStack w={"100%"***REMOVED*** h={"100%"***REMOVED*** p={4***REMOVED*** overflow={"auto"***REMOVED***>
                        ***REMOVED***tags.length > 0 && (<Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Tags</Text>)***REMOVED***
                            <Flex flexWrap={"wrap"***REMOVED***>
                            ***REMOVED***tags.map((tag) =>
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
                        ***REMOVED***tags.length > 0 && (<Divider/>)***REMOVED***
                        ***REMOVED***albums.length > 0 && (<Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Albums</Text>)***REMOVED***
                            <Flex flexWrap={"wrap"***REMOVED***>
                            ***REMOVED***albums.map((album) =>
                                    <Tag
                                        size={"md"***REMOVED***
                                        key={album***REMOVED***
                                        borderRadius='full'
                                        variant='solid'
                                        colorScheme='gray'
                                        m={1***REMOVED***
                                    >
                                        <TagLabel>{album***REMOVED***</TagLabel>
                                    </Tag>
                                )***REMOVED***
                            </Flex>
                        ***REMOVED***albums.length > 0 && (<Divider/>)***REMOVED***
                            <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Metadata</Text>
                            <HStack justifyContent={"space-between"***REMOVED*** alignItems={"flex-start"***REMOVED*** overflow={"auto"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Name: </Text>
                                <Text width={'80%'***REMOVED***>{pname***REMOVED***</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Upload Date: </Text>
                                <Text>{date***REMOVED***</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"***REMOVED*** w={"100%"***REMOVED***>
                                <Text>Path: </Text>
                                <Text>{path***REMOVED***</Text>
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