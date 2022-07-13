import {Box, Image, ImageProps, useDisclosure, Text, Center***REMOVED*** from "@chakra-ui/react"
import {ContextMenu***REMOVED*** from "chakra-ui-contextmenu"
import ImageContextMenu from "./contextMenus/ImageContextMenu"
import ImageView from "./ImageView"

const ImageItem = (props: Exclude<ImageProps, "onClick" | "borderRadius">) => {
    const {isOpen, onOpen, onClose***REMOVED*** = useDisclosure()
    return (
        <ContextMenu<HTMLImageElement>
            renderMenu={() => <ImageContextMenu/>***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w={64***REMOVED*** overflow='hidden' position='relative' m={2***REMOVED*** rounded={"md"***REMOVED*** border={"1px"***REMOVED***
                     borderColor={"gray.50"***REMOVED*** shadow={"md"***REMOVED***>
                    <Image {...props***REMOVED*** onClick={onOpen***REMOVED*** alt={"image"***REMOVED***/>
                    <Center bg={"white"***REMOVED***>
                        <Text fontWeight={"hairline"***REMOVED***>filename.jpg</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen***REMOVED*** onViewClose={onClose***REMOVED***/>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default ImageItem