import {Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {BiChevronRight***REMOVED*** from "react-icons/bi";
import React from "react";

interface PathbarProps {
    children?: React.ReactNode
***REMOVED***
const Pathbar: React.FC<PathbarProps> = ({children***REMOVED***) => {

    return (
        <VStack pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED***/>
                <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>/</Text>
            </HStack>
            <Divider/>
        ***REMOVED***children***REMOVED***
        </VStack>
    )
***REMOVED***
export default Pathbar