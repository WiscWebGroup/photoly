import {Divider, HStack, Icon, Text, VStack} from "@chakra-ui/react";
import {BiChevronRight} from "react-icons/bi";
import React from "react";

interface PathbarProps {
    children?: React.ReactNode
}
const Pathbar: React.FC<PathbarProps> = ({children}) => {

    return (
        <VStack pt={8} pl={8} w={"100%"} pr={8}>
            <HStack alignSelf={"flex-start"}>
                <Icon as={BiChevronRight} boxSize={8} color={"gray.500"}/>
                <Text fontWeight={"bold"} color={"gray.500"}>/</Text>
            </HStack>
            <Divider/>
            {children}
        </VStack>
    )
}
export default Pathbar