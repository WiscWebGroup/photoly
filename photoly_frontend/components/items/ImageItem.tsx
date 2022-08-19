import {Box, Center, Image, ImageProps, Text, useDisclosure} from "@chakra-ui/react"
import {ContextMenu} from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import ImageView from "../ImageView"
import {useEffect, useState} from "react";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";

interface ImageItemProps {
    pid: number
    name: string
    format: string
    uploaddate: string
    orgsrc: string
}
interface TagLookup{
    photo_id: number
    tag_name: string
    tag_id: number
}

const ImageItem = (props: Exclude<ImageProps & ImageItemProps, "onClick" | "borderRadius">) => {
    const {isOpen, onOpen, onClose} = useDisclosure()
    const [tags, setTags] = useState<string[]>([]);
    const {get} = useApi()
    const token = useToken()

    useEffect(() => {
        const getTags = async () => {
            await get(`/photo/getTagByPhoto?photoId=${props.pid}`, {
                headers: {"HRD-token": token}
            }).then((response) => {
                if (!!response && response.data && response.data.msgCode === 200) {
                    const temp: string[] = []
                    response.data.t.forEach((item: TagLookup) => {
                        temp.push(item.tag_name)
                    })
                    setTags(temp)
                }
            })

        }
        if (token) getTags()
    }, [token])

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
                    <ImageView isViewOpen={isOpen} onViewClose={onClose} path={props.src} pname={props.name}
                               date={props.uploaddate} orgsrc={props.orgsrc} tags={tags} albums={[]}/>
                </Box>
            )}
        </ContextMenu>
    )
}

export default ImageItem