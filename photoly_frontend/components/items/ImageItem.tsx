import {Box, Image, ImageProps, useDisclosure, Text, Center***REMOVED*** from "@chakra-ui/react"
import { ContextMenu ***REMOVED*** from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import { useSearchData ***REMOVED*** from "../contexts/SearchContext"
import ImageView from "../ImageView"

interface ImageItemProps {
    pid: number
    name: string
    format: string
***REMOVED***

const ImageItem = (props: Exclude<ImageProps & ImageItemProps, "onClick" | "borderRadius">) => {
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()
    

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => <ImageContextMenu/>***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w={64***REMOVED*** overflow='hidden' position='relative' m={2***REMOVED*** rounded={"md"***REMOVED*** border={"1px"***REMOVED***
                     borderColor={"gray.50"***REMOVED*** shadow={"md"***REMOVED***>
                    <Image 
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***...props***REMOVED***
                        w="100%" 
                        h={128***REMOVED*** 
                        objectFit="cover" 
                        onClick={onOpen***REMOVED*** 
                        alt={"image"***REMOVED***
                    />
                    <Center bg={"white"***REMOVED***>
                        <Text fontWeight={"hairline"***REMOVED***>{props.name + "." + props.format***REMOVED***</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen***REMOVED*** onViewClose={onClose***REMOVED***/>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default ImageItem