import {Box, Divider, Flex, HStack, Icon, Image, Text, VStack} from "@chakra-ui/react";
import {BiChevronRight} from "react-icons/bi";

export default function Pathbar(){
    return (
        <VStack pt={8} pl={8} w={"100%"} pr={8}>
            <HStack alignSelf={"flex-start"}>
                <Icon as={BiChevronRight} boxSize={8} color={"gray.500"}/>
                <Text fontWeight={"bold"} color={"gray.500"}>/home</Text>
            </HStack>
            <Divider/>
            <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                <Box maxW='sm' overflow='hidden' p={2}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
            </Flex>


        </VStack>
    )
}