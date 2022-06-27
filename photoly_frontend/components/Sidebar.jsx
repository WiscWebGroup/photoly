import {Button, Divider, HStack, Icon, Text, VStack} from "@chakra-ui/react";
import {AiFillHeart, AiFillTags} from "react-icons/ai";
import TagItem from "./TagItem";
import {BiBaseball, BiPhotoAlbum} from "react-icons/bi";

export default function Sidebar(){
    const tags = ["Animals", "Cute", "Furry", "SM", "BL", "🔞", "White hair", "Monster ear"]
    return (
        <VStack w={52} mt={4} ml={4} rounded={"lg"} bg={"blackAlpha.50"} p={4} boxShadow={"lg"}>
            <HStack alignSelf={"flex-start"}>
                <Text fontSize={"xl"}>Tags & Albums</Text>
            </HStack>

            <Divider/>

            <HStack alignSelf={"flex-start"}>
                <Icon as={AiFillTags} color={"teal.400"}></Icon>
                <Text>Your Tags</Text>
            </HStack>


            {tags.map((tag) =>
                <TagItem tagName={tag} key={tag}></TagItem>
            )}

            <Divider/>

            <HStack alignSelf={"flex-start"}>
                <Icon as={BiPhotoAlbum} color={"teal.400"}></Icon>
                <Text>Your Albums</Text>
            </HStack>

            <Button variant={"ghost"} w={"100%"} justifyContent={"flex-start"}
                    leftIcon={<Icon as={AiFillHeart} color={"red.400"}/>} fontWeight={"normal"} isActive={""}>My
                Favorite</Button>
            <Button variant={"ghost"} w={"100%"} justifyContent={"flex-start"}
                    leftIcon={<Icon as={BiBaseball} color={"red.400"}/>} fontWeight={"normal"}>
                Ye's Favorite</Button>
        </VStack>
    )
}