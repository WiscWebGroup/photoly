import {Button, Divider, HStack, Icon, Text, VStack} from "@chakra-ui/react";
import {AiFillHeart, AiFillTags} from "react-icons/ai";
import TagItem from "./TagItem";
import {BiBaseball, BiPhotoAlbum} from "react-icons/bi";
import React from "react";
import TagHeader from "./TagHeader";

const Sidebar: React.FC = () => {
    const tags = ["Animals", "Cute", "Furry", "SM", "BL", "🔞", "White hair", "Monster ear"]

    const addNewTag = () => {
        console.log("DEBUG/TODO: add a new tag to the sidebar (use backend api).")
    }

    const addNewCategory = () => {
        console.log("DEBUG/TODO: add a new category to the sidebar (use backend api).")
    }

    return (
        <VStack w={52} mt={4} ml={4} rounded={"lg"} bg={"blackAlpha.50"} p={4} boxShadow={"lg"}>
            <HStack alignSelf={"flex-start"}>
                <Text fontSize={"xl"}>Tags & Albums</Text>
            </HStack>

            <Divider/>
            
            <TagHeader 
                headerIcon={AiFillTags} 
                iconColor="teal.400"
                onAdd={addNewTag}
            >
                Your Tags
            </TagHeader>


            {tags.map((tag, ind) =>
                <TagItem tagName={tag} key={ind}></TagItem>
            )}

            <Divider/>

            <TagHeader 
                headerIcon={BiPhotoAlbum} 
                iconColor="teal.400"
                onAdd={addNewCategory}
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