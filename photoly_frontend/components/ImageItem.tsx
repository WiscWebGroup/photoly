import { Box, Image, ImageProps, useDisclosure } from "@chakra-ui/react"
import { ContextMenu } from "chakra-ui-contextmenu"
import ImageContextMenu from "./contextMenus/ImageContextMenu"
import ImageView from "./ImageView"

const ImageItem = (props: Exclude<ImageProps, "onClick" | "borderRadius">) => {
    const { isOpen, onOpen, onClose } = useDisclosure()
    return (
        <ContextMenu<HTMLImageElement>
            renderMenu={() => <ImageContextMenu />}
        >
            {ref => (
                <Box ref={ref} maxW='sm' overflow='hidden' position='relative' p={2}>
                    <Image {...props} borderRadius='lg' onClick={onOpen}/>
                    <ImageView isViewOpen={isOpen} onViewClose={onClose}/>
                </Box>
            )}
        </ContextMenu>
    )
}

export default ImageItem