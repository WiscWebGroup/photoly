import {Button, Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {AiFillHeart***REMOVED*** from "react-icons/ai";
import {BiBaseball, BiPhotoAlbum***REMOVED*** from "react-icons/bi";
import React from "react";
import TagHeader from "./TagHeader";
import TagContextProvider from "./contexts/TagContext";
import TagSection from "./TagSection";

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

            <TagHeader 
                headerIcon={BiPhotoAlbum***REMOVED*** 
                iconColor="teal.400"
            >
                Your Albums
            </TagHeader>

            <Button variant={"ghost"***REMOVED*** w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED***
                    leftIcon={<Icon as={AiFillHeart***REMOVED*** color={"red.400"***REMOVED***/>***REMOVED*** fontWeight={"normal"***REMOVED*** isActive={false***REMOVED***>My
                Favorite</Button>
            <Button variant={"ghost"***REMOVED*** w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED***
                    leftIcon={<Icon as={BiBaseball***REMOVED*** color={"red.400"***REMOVED***/>***REMOVED*** fontWeight={"normal"***REMOVED***>
                Yes Favorite</Button>
        </VStack>
    )
***REMOVED***

export default Sidebar