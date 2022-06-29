import {Checkbox} from "@chakra-ui/react";

interface props {
    tagName: string
}

export default function TagItem({tagName}:props){
    return (
        <Checkbox variant={"ghost"} w={"100%"} pl={4}>{tagName}</Checkbox >
    )
}