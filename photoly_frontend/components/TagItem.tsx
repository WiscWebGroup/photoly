import {Checkbox} from "@chakra-ui/react";
import React from "react";

interface TagItemProps {
    tagName: string
}

 const TagItem: React.FC<TagItemProps> = ({tagName}) => {
    return (
        <Checkbox variant={"ghost"} w={"100%"} pl={4}>{tagName}</Checkbox >
    )
}
export default TagItem