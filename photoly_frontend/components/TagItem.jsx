import {Checkbox} from "@chakra-ui/react";

export default function TagItem({tagName}){
    return (
        <Checkbox variant={"ghost"} w={"100%"} pl={4}>{tagName}</Checkbox >
    )
}