import {Divider, Heading, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {BiChevronRight***REMOVED*** from "react-icons/bi";

export default function Pathbar(){
    return (
        <VStack alignSelf={"flex-start"***REMOVED*** pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED***/>
                <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>/home</Text>
            </HStack>
            <Divider/>
            <Heading>Photo Here</Heading>
        </VStack>
    )
***REMOVED***