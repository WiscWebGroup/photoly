import {Box, Center, Image, ImageProps, Text, useDisclosure***REMOVED*** from "@chakra-ui/react"
import {ContextMenu***REMOVED*** from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import ImageView from "../ImageView"
import {useEffect, useState***REMOVED*** from "react";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";

interface ImageItemProps {
    pid: number
    name: string
    format: string
    uploaddate: string
    orgsrc: string
    folder_name: string
***REMOVED***
interface TagLookup{
    photo_id: number
    tag_name: string
    tag_id: number
***REMOVED***
interface GalleryLookup{
    ga_id: number
    ga_name: string
    photo_id: number
***REMOVED***

const ImageItem = (props: Exclude<ImageProps & ImageItemProps, "onClick" | "borderRadius">) => {
    const {isOpen, onOpen, onClose***REMOVED*** = useDisclosure()
    const [tags, setTags] = useState<string[]>([]);
    const [galleries, setGalleries] = useState<string[]>([]);
    const {get***REMOVED*** = useApi()
    const token = useToken()

    useEffect(() => {
        const getTags = async () => {
            await get(`/photo/getTagByPhoto?photoId=${props.pid***REMOVED***`, {
                headers: {"HRD-token": token***REMOVED***
    ***REMOVED***).then((response) => {
                if (!!response && response.data && response.data.msgCode === 200) {
                    const temp: string[] = []
                    response.data.t.forEach((item: TagLookup) => {
                        temp.push(item.tag_name)
  ***REMOVED*****REMOVED*****REMOVED***)
                    setTags(temp)
   ***REMOVED*****REMOVED***
    ***REMOVED***)
***REMOVED***
        const getGalleries = async () => {
            await get(`/photo/getGalleryByPhoto?photoId=${props.pid***REMOVED***`, {
                headers: {"HRD-token": token***REMOVED***
    ***REMOVED***).then((response) => {
                if (!!response && response.data && response.data.msgCode === 200) {
                    const temp: string[] = []
                    response.data.t.forEach((item: GalleryLookup) => {
                        temp.push(item.ga_name)
  ***REMOVED*****REMOVED*****REMOVED***)
                    setGalleries(temp)
   ***REMOVED*****REMOVED***
    ***REMOVED***)
***REMOVED***
        if (token){
            getTags();
            getGalleries();
***REMOVED***
***REMOVED***, [token])

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
                    <ImageView isViewOpen={isOpen***REMOVED*** onViewClose={onClose***REMOVED*** path={props.folder_name***REMOVED*** pname={props.name***REMOVED***
                               date={props.uploaddate***REMOVED*** orgsrc={props.orgsrc***REMOVED*** tags={tags***REMOVED*** albums={galleries***REMOVED***/>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default ImageItem