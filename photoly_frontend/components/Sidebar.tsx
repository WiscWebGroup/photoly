import {Button, Divider, HStack, Icon, Text, VStack***REMOVED*** from "@chakra-ui/react";
import {AiFillHeart, AiFillTags***REMOVED*** from "react-icons/ai";
import TagItem from "./TagItem";
import {BiBaseball, BiPhotoAlbum***REMOVED*** from "react-icons/bi";
import React from "react";

const Sidebar: React.FC = () => {
    const tags = ["Animals", "Cute", "Furry", "SM", "BL", "🔞", "White hair", "Monster ear"]
    return (
        <VStack w={52***REMOVED*** mt={4***REMOVED*** ml={4***REMOVED*** rounded={"lg"***REMOVED*** bg={"blackAlpha.50"***REMOVED*** p={4***REMOVED*** boxShadow={"lg"***REMOVED***>
            <HStack alignSelf={"flex-start"***REMOVED***>
                <Text fontSize={"xl"***REMOVED***>Tags & Albums</Text>
            </HStack>

            <Divider/>

            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={AiFillTags***REMOVED*** color={"teal.400"***REMOVED***></Icon>
                <Text>Your Tags</Text>
            </HStack>


   ***REMOVED*****REMOVED***tags.map((tag) =>
                <TagItem tagName={tag***REMOVED*** key={tag***REMOVED***></TagItem>
            )***REMOVED***

            <Divider/>

            <HStack alignSelf={"flex-start"***REMOVED***>
                <Icon as={BiPhotoAlbum***REMOVED*** color={"teal.400"***REMOVED***></Icon>
                <Text>Your Albums</Text>
            </HStack>

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