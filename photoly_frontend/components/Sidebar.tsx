import {Divider, HStack, Text, VStack} from "@chakra-ui/react";
import React from "react";
import TagContextProvider from "./contexts/TagContext";
import TagSection from "./TagSection";
import AlbumContextProvider from "./contexts/AlbumContext";
import AlbumSection from "./AlbumSection";

const Sidebar: React.FC = () => {
    return (
        <VStack w={52} mt={4} ml={4} rounded={"lg"} bg={"blackAlpha.50"} p={4} boxShadow={"lg"}>
            <HStack alignSelf={"flex-start"}>
                <Text fontSize={"xl"}>Tags & Albums</Text>
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
}

export default Sidebar