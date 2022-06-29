import {Box, Divider, Flex, HStack, Icon, Image, Text, VStack} from "@chakra-ui/react";
import {BiChevronRight} from "react-icons/bi";
import {useCallback, useEffect, useState} from "react";
import ImageContext from "./ImageContext";

export default function Pathbar(){
    const [show, setShow] = useState(false)
    const [anchorPoint, setAnchorPoint] = useState({ x: 0, y: 0 })
    const handleClick = useCallback(() => (show ? setShow(false) : null), [show])
    const handleContextMenu = (e) => {
        e.preventDefault()
        setAnchorPoint({ x: e.pageX, y: e.pageY })
        setShow(true)
    }
    useEffect(() => {
        document.addEventListener("click", handleClick);
        return () => {
            document.removeEventListener("click", handleClick);
        }
    })
    return (
        <VStack pt={8} pl={8} w={"100%"} pr={8}>
            <HStack alignSelf={"flex-start"}>
                <Icon as={BiChevronRight} boxSize={8} color={"gray.500"}/>
                <Text fontWeight={"bold"} color={"gray.500"}>/</Text>
            </HStack>
            <Divider/>
            <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
            </Flex>
            {show && <ImageContext x={anchorPoint.x} y={anchorPoint.y}/>}
        </VStack>
    )
}