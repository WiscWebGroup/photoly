import {Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {BiChevronRight***REMOVED*** from "react-icons/bi";
import React from "react";
import {useSearchData***REMOVED*** from "./contexts/SearchContext";

interface PathbarProps {
    children?: React.ReactNode
***REMOVED***
const Pathbar: React.FC<PathbarProps> = ({children***REMOVED***) => {
    const searchData = useSearchData();

    return (
        <VStack pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED***/>
                <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>{searchData.path.map((f) => {if (f.name !== "/") return f.name***REMOVED***).join("/")***REMOVED***</Text>
            </HStack>
            <Divider/>
   ***REMOVED*****REMOVED***children***REMOVED***
        </VStack>
    )
***REMOVED***
export default Pathbar