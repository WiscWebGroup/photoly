import React, {Dispatch, SetStateAction***REMOVED*** from "react";
import {Box, Divider, Flex, HStack, Image, Tag, TagLabel, Text, useOutsideClick, VStack***REMOVED*** from "@chakra-ui/react";

interface ImageViewProps {
    isModalOpen: boolean,
    setIsModelOpen: Dispatch<SetStateAction<boolean>>
***REMOVED***

const ImageView: React.FC<ImageViewProps> = ({isModalOpen, setIsModelOpen***REMOVED***) => {
    const ref = React.useRef() as React.MutableRefObject<HTMLDivElement>
    useOutsideClick({
        ref: ref,
        handler: () => setIsModelOpen(false),
***REMOVED***)
    const tags = ["Animals", "Cute", "Furry", "SM", "BL", "🔞", "White hair", "Monster ear"]
    return (
        <>
   ***REMOVED*****REMOVED***isModalOpen && (
                <Box w={"100vw"***REMOVED*** h={"100vh"***REMOVED*** position={"fixed"***REMOVED*** bg={"gray.100"***REMOVED*** zIndex={"-1"***REMOVED*** top={"50%"***REMOVED*** left={"50%"***REMOVED***
                     transform={"translate(-50%, -50%)"***REMOVED***></Box>
            )***REMOVED***
   ***REMOVED*****REMOVED***isModalOpen && (
                <HStack position={"fixed"***REMOVED*** w={"1600px"***REMOVED*** h={"800px"***REMOVED*** rounded={"lg"***REMOVED*** bg={"gray.50"***REMOVED*** top={"50%"***REMOVED***
                        left={"50%"***REMOVED***
                        transform={"translate(-50%, -50%)"***REMOVED*** ref={ref***REMOVED***>
                    <Image src={"https://www.industrialempathy.com/img/remote/ZiClJf-1920w.jpg"***REMOVED*** alt={"image"***REMOVED***
                           rounded={"10px"***REMOVED***
                           w={"80%"***REMOVED*** h={"100%"***REMOVED*** objectFit={"fill"***REMOVED***/>
                    <VStack w={"100%"***REMOVED*** h={"100%"***REMOVED*** p={4***REMOVED***>
                        <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED***>Tags</Text>
                        <Flex flexWrap={"wrap"***REMOVED***>
    ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***tags.map((tag) =>
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
    ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***tags.map((tag) =>
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
            )
    ***REMOVED***
        </>
    )
***REMOVED***

export default ImageView