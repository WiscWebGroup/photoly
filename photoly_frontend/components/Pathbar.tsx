import {Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {BiChevronRight***REMOVED*** from "react-icons/bi";
import {ReactNode***REMOVED*** from "react";

interface PathbarProps {
    children?: ReactNode
***REMOVED***
export default function Pathbar({children***REMOVED***: PathbarProps){

    return (
        <VStack pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED***/>
                <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>/</Text>
            </HStack>
            <Divider/>
   ***REMOVED*****REMOVED***children***REMOVED***
        </VStack>
    )
***REMOVED***