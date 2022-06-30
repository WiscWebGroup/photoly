import {Box, Divider, Flex, HStack, Icon, Image, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {BiChevronRight***REMOVED*** from "react-icons/bi";
import {useCallback, useEffect, useState***REMOVED*** from "react";
import ImageContext from "./ImageContext";

export default function Pathbar(){
    const [show, setShow] = useState(false)
    const [anchorPoint, setAnchorPoint] = useState({ x: 0, y: 0 ***REMOVED***)
    const handleClick = useCallback(() => (show ? setShow(false) : null), [show])
    const handleContextMenu = (e: MouseEvent) => {
        e.preventDefault()
        setAnchorPoint({ x: e.pageX, y: e.pageY ***REMOVED***)
        setShow(true)
***REMOVED***
    useEffect(() => {
        document.addEventListener("click", handleClick);
        return () => {
            document.removeEventListener("click", handleClick);
***REMOVED***
***REMOVED***)
    return (
        <VStack pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED***/>
                <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>/</Text>
            </HStack>
            <Divider/>
            <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={() => handleContextMenu***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2***REMOVED*** onContextMenu={() => handleContextMenu***REMOVED***>
                    <Image src={"https://bit.ly/2Z4KKcF"***REMOVED*** alt={"image"***REMOVED*** borderRadius='lg'/>
                </Box>
            </Flex>
   ***REMOVED*****REMOVED***show && <ImageContext x={anchorPoint.x***REMOVED*** y={anchorPoint.y***REMOVED***/>***REMOVED***
        </VStack>
    )
***REMOVED***