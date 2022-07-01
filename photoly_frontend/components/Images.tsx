import {Box, Flex, Image} from "@chakra-ui/react";
import ImageContextMenu from "./ImageContextMenu";
import {useContextMenu} from "../hooks/useContextMenu";
import React from "react";
import ImageView from "./ImageView";

const Images: React.FC = () => {
    const [showContext, handleContextMenu, anchorPoint] = useContextMenu()
    const [isModalOpen, setIsModalOpen] = React.useState(true)
    return (
        <>
            <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg' onClick={() => setIsModalOpen(true)}/>
                </Box>
                <Box maxW='sm' overflow='hidden' p={2} onContextMenu={handleContextMenu} onClick={() => setIsModalOpen(true)}>
                    <Image src={"https://bit.ly/2Z4KKcF"} alt={"image"} borderRadius='lg'/>
                </Box>
            </Flex>
            {showContext && <ImageContextMenu x={anchorPoint.x} y={anchorPoint.y}/>}
            <ImageView isModalOpen={isModalOpen} setIsModelOpen={setIsModalOpen}/>
        </>

    )
}
export default Images