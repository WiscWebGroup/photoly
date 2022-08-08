import {Box, Image, ImageProps, useDisclosure, Text, Center} from "@chakra-ui/react"
import { ContextMenu } from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import { useSearchData } from "../contexts/SearchContext"
import ImageView from "../ImageView"

interface ImageItemProps {
    pid: number
    name: string
    format: string
    uploadDate: string
}

const ImageItem = (props: Exclude<ImageProps & ImageItemProps, "onClick" | "borderRadius">) => {
    const { isOpen, onOpen, onClose } = useDisclosure()
    

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => <ImageContextMenu/>}
        >
            {ref => (
                <Box ref={ref} w={64} overflow='hidden' position='relative' m={2} rounded={"md"} border={"1px"}
                     borderColor={"gray.50"} shadow={"md"}>
                    <Image 
                        {...props}
                        w="100%" 
                        h={128} 
                        objectFit="cover" 
                        onClick={onOpen} 
                        alt={"image"}
                    />
                    <Center bg={"white"}>
                        <Text fontWeight={"hairline"}>{props.name + "." + props.format}</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen} onViewClose={onClose} path={props.src} pname={props.name} date={props.uploadDate}/>
                </Box>
            )}
        </ContextMenu>
    )
}

export default ImageItem