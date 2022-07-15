import {Button, Divider, HStack, Icon, Text, VStack} from "@chakra-ui/react";
import {AiFillHeart} from "react-icons/ai";
import {BiBaseball, BiPhotoAlbum} from "react-icons/bi";
import React from "react";
import TagHeader from "./TagHeader";
import TagContextProvider from "./contexts/TagContext";
import TagSection from "./TagSection";

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

            <TagHeader 
                headerIcon={BiPhotoAlbum} 
                iconColor="teal.400"
            >
                Your Albums
            </TagHeader>

            <Button variant={"ghost"} w={"100%"} justifyContent={"flex-start"}
                    leftIcon={<Icon as={AiFillHeart} color={"red.400"}/>} fontWeight={"normal"} isActive={false}>My
                Favorite</Button>
            <Button variant={"ghost"} w={"100%"} justifyContent={"flex-start"}
                    leftIcon={<Icon as={BiBaseball} color={"red.400"}/>} fontWeight={"normal"}>
                Yes Favorite</Button>
        </VStack>
    )
}

export default Sidebar