import {Divider, HStack, Text, VStack***REMOVED*** from "@chakra-ui/react";
import React from "react";
import TagContextProvider from "./contexts/TagContext";
import TagSection from "./TagSection";
import AlbumContextProvider from "./contexts/AlbumContext";
import AlbumSection from "./AlbumSection";

const Sidebar: React.FC = () => {
    return (
        <VStack w={52***REMOVED*** mt={4***REMOVED*** ml={4***REMOVED*** rounded={"lg"***REMOVED*** bg={"blackAlpha.50"***REMOVED*** p={4***REMOVED*** boxShadow={"lg"***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Text fontSize={"xl"***REMOVED***>Tags & Albums</Text>
            </HStack>

            <Divider/>
            
            <TagContextProvider>
                <TagSection />
            </TagContextProvider>

            <Divider/>

            <AlbumContextProvider>
                <AlbumSection />
            </AlbumContextProvider>
        </VStack>
    )
***REMOVED***

export default Sidebar