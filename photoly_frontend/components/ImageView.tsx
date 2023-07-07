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
} from "@chakra-ui/react";

interface ImageViewProps {
    isViewOpen: boolean,
    onViewClose: () => void,
    path: string | undefined,
    pname: string | undefined,
    date: string | undefined,
    orgsrc: string,
    tags: string[],
    albums: string[]
}

const ImageView: React.FC<ImageViewProps> = ({isViewOpen, onViewClose, path, pname, date, orgsrc, tags, albums}) => {
    const ref = React.useRef() as React.MutableRefObject<HTMLDivElement>
    useOutsideClick({
        ref: ref,
        handler: onViewClose,
    })
    return (
        <>
            <Modal isOpen={isViewOpen} onClose={onViewClose}>
                <ModalOverlay/>
                <ModalContent maxW={"85vw"}>
                    <ModalCloseButton/>
                    <HStack alignItems={"flex-start"}>
                        <Image src={orgsrc} alt={"image"}
                               rounded={"md"}
                               w={"80%"} h={"100%"} objectFit={"fill"}/>
                        <VStack w={"100%"} h={"100%"} p={4} overflow={"auto"}>
                            {tags.length > 0 && (<Text fontWeight={"semibold"} fontSize={"xl"}>Tags</Text>)}
                            <Flex flexWrap={"wrap"}>
                                {tags.map((tag) =>
                                    <Tag
                                        size={"md"}
                                        key={tag}
                                        borderRadius='full'
                                        variant='solid'
                                        colorScheme='teal'
                                        m={1}
                                    >
                                        <TagLabel>{tag}</TagLabel>
                                    </Tag>
                                )}
                            </Flex>
                            {tags.length > 0 && (<Divider/>)}
                            {albums.length > 0 && (<Text fontWeight={"semibold"} fontSize={"xl"}>Albums</Text>)}
                            <Flex flexWrap={"wrap"}>
                                {albums.map((album) =>
                                    <Tag
                                        size={"md"}
                                        key={album}
                                        borderRadius='full'
                                        variant='solid'
                                        colorScheme='gray'
                                        m={1}
                                    >
                                        <TagLabel>{album}</TagLabel>
                                    </Tag>
                                )}
                            </Flex>
                            {albums.length > 0 && (<Divider/>)}
                            <Text fontWeight={"semibold"} fontSize={"xl"}>Metadata</Text>
                            <HStack justifyContent={"space-between"} alignItems={"flex-start"} overflow={"auto"} w={"100%"}>
                                <Text>Name: </Text>
                                <Text width={'80%'}>{pname}</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"} w={"100%"}>
                                <Text>Upload Date: </Text>
                                <Text>{date}</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"} w={"100%"}>
                                <Text>Path: </Text>
                                <Text>{path}</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"} w={"100%"}>
                                <Text>Format: </Text>
                                <Text>png</Text>
                            </HStack>
                            <HStack justifyContent={"space-between"} w={"100%"}>
                                <Text>Shared: </Text>
                                <Text>No</Text>
                            </HStack>

                        </VStack>
                    </HStack>
                </ModalContent>
            </Modal>
        </>
    )
}

export default ImageView