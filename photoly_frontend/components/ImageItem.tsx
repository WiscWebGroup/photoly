import {Box, Image, ImageProps, useDisclosure, Text, Center} from "@chakra-ui/react"
import {ContextMenu} from "chakra-ui-contextmenu"
import ImageContextMenu from "./contextMenus/ImageContextMenu"
import ImageView from "./ImageView"

const ImageItem = (props: Exclude<ImageProps, "onClick" | "borderRadius">) => {
    const {isOpen, onOpen, onClose} = useDisclosure()
    return (
        <ContextMenu<HTMLImageElement>
            renderMenu={() => <ImageContextMenu/>}
        >
            {ref => (
                <Box ref={ref} w={64} overflow='hidden' position='relative' m={2} rounded={"md"} border={"1px"}
                     borderColor={"gray.50"} shadow={"md"}>
                    <Image {...props} onClick={onOpen} alt={"image"}/>
                    <Center bg={"white"}>
                        <Text fontWeight={"hairline"}>filename.jpg</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen} onViewClose={onClose}/>
                </Box>
            )}
        </ContextMenu>
    )
}

export default ImageItem