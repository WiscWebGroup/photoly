import { Box, Image, ImageProps, useDisclosure ***REMOVED*** from "@chakra-ui/react"
import { ContextMenu ***REMOVED*** from "chakra-ui-contextmenu"
import ImageContextMenu from "./contextMenus/ImageContextMenu"
import ImageView from "./ImageView"

const ImageItem = (props: Exclude<ImageProps, "onClick" | "borderRadius">) => {
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()
    return (
        <ContextMenu<HTMLImageElement>
            renderMenu={() => <ImageContextMenu />***REMOVED***
        >
        ***REMOVED***ref => (
                <Box ref={ref***REMOVED*** maxW='sm' overflow='hidden' position='relative' p={2***REMOVED***>
                    <Image {...props***REMOVED*** borderRadius='lg' onClick={onOpen***REMOVED*** alt={"image"***REMOVED***/>
                    <ImageView isViewOpen={isOpen***REMOVED*** onViewClose={onClose***REMOVED***/>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default ImageItem