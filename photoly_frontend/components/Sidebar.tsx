import {Button, Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {AiFillHeart, AiFillTags***REMOVED*** from "react-icons/ai";
import TagItem from "./TagItem";
import {BiBaseball, BiPhotoAlbum***REMOVED*** from "react-icons/bi";
import React from "react";
import TagHeader from "./TagHeader";

const Sidebar: React.FC = () => {
    const tags = ["Animals", "Cute", "Furry", "White hair", "Landscape"]

    const addNewTag = () => {
        console.log("DEBUG/TODO: add a new tag to the sidebar (use backend api).")
***REMOVED***

    const addNewCategory = () => {
        console.log("DEBUG/TODO: add a new category to the sidebar (use backend api).")
***REMOVED***

    return (
        <VStack w={52***REMOVED*** mt={4***REMOVED*** ml={4***REMOVED*** rounded={"lg"***REMOVED*** bg={"blackAlpha.50"***REMOVED*** p={4***REMOVED*** boxShadow={"lg"***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Text fontSize={"xl"***REMOVED***>Tags & Albums</Text>
            </HStack>

            <Divider/>
            
            <TagHeader 
                headerIcon={AiFillTags***REMOVED*** 
                iconColor="teal.400"
                onAdd={addNewTag***REMOVED***
            >
                Your Tags
            </TagHeader>


        ***REMOVED***tags.map((tag, ind) =>
                <TagItem tagName={tag***REMOVED*** key={ind***REMOVED***></TagItem>
            )***REMOVED***

            <Divider/>

            <TagHeader 
                headerIcon={BiPhotoAlbum***REMOVED*** 
                iconColor="teal.400"
                onAdd={addNewCategory***REMOVED***
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