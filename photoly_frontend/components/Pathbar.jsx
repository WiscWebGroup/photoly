import {Divider, Heading, HStack, Icon, Text, VStack} from "@chakra-ui/react";
import {BiChevronRight} from "react-icons/bi";

export default function Pathbar(){
    return (
        <VStack alignSelf={"flex-start"} pt={8} pl={8} w={"100%"} pr={8}>
            <HStack alignSelf={"flex-start"}>
                <Icon as={BiChevronRight} boxSize={8} color={"gray.500"}/>
                <Text fontWeight={"bold"} color={"gray.500"}>/home</Text>
            </HStack>
            <Divider/>
            <Heading>Photo Here</Heading>
        </VStack>
    )
}