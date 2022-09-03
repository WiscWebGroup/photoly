import {Box, Button, Center, FormControl, FormErrorMessage, FormLabel, Image, ImageProps, Input, Link, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast***REMOVED*** from "@chakra-ui/react"
import {ContextMenu***REMOVED*** from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import ImageView from "../ImageView"
import {ChangeEvent, useEffect, useRef, useState***REMOVED*** from "react";
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
    const {get, post***REMOVED*** = useApi()
    const token = useToken()
    const toast = useToast();

    const [newName, setNewName] = useState('')
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const {isOpen: isRenameOpen, onOpen: onRenameOpen, onClose: onRenameClose***REMOVED*** = useDisclosure()

    const aRef = useRef<HTMLAnchorElement>(null)

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

    const deleteRequest = async () => {
        const response = await post("/photo/delete", null, {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
              photoId: props.pid,
    ***REMOVED***,
***REMOVED***);

        if (!!response && response.data && response.data.msgCode === 200) {
            // TODO: refresh
***REMOVED*** else {
            toast({
                title: `Failed to delete the photo "${props.name***REMOVED***.${props.format***REMOVED******REMOVED***"`,
                status: 'error',
                isClosable: true,
                position: 'top'
    ***REMOVED***)
***REMOVED***
***REMOVED***

    const renameRequest = async () => {
        setIsLoading(true)
        const response = await post("/photo/update", 
   ***REMOVED*****REMOVED***
                photoId: props.pid,
                photoName: newName,
    ***REMOVED***, 
   ***REMOVED*****REMOVED***
                headers: { "HRD-token": token ***REMOVED***
    ***REMOVED***
        );

        if (!!response && response.data && response.data.msgCode === 200) {
            // TODO: refresh
***REMOVED*** else {
            toast({
                title: `Failed to rename the photo "${props.name***REMOVED***.${props.format***REMOVED******REMOVED***"`,
                status: 'error',
                isClosable: true,
                position: 'top'
    ***REMOVED***)
***REMOVED***
        setIsLoading(false);
        handleResetNewName();
***REMOVED***

    const handleResetNewName = () => {
        setNewName('')
        setIsError(true)
        onRenameClose()
***REMOVED***

    const handlePhotoRename = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value;
        setIsError(value.length === 0);
        setNewName(value);
***REMOVED***

    const handleDownload = () => {
        aRef.current?.click();
***REMOVED***

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => (
                <ImageContextMenu 
                    handleDelete={deleteRequest***REMOVED***
                    handleView={onOpen***REMOVED***
                    handleDownload={handleDownload***REMOVED***
                    handleRename={onRenameOpen***REMOVED***
                    handleMoveTo={() => {***REMOVED******REMOVED***
                />
            )***REMOVED***
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
                    <Link ref={aRef***REMOVED*** href={props.orgsrc***REMOVED*** display="none" download />
                    <Center bg={"white"***REMOVED*** w={"100%"***REMOVED***>
                        <Text fontWeight={"hairline"***REMOVED*** whiteSpace={"nowrap"***REMOVED*** overflow={"hidden"***REMOVED*** textOverflow={"ellipsis"***REMOVED***>{props.name + "." + props.format***REMOVED***</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen***REMOVED*** onViewClose={onClose***REMOVED*** path={props.folder_name***REMOVED*** pname={props.name***REMOVED***
                               date={props.uploaddate***REMOVED*** orgsrc={props.orgsrc***REMOVED*** tags={tags***REMOVED*** albums={galleries***REMOVED***/>
                    
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***/* change name modal***REMOVED******REMOVED***
                    <Modal isOpen={isRenameOpen***REMOVED*** onClose={handleResetNewName***REMOVED***>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Rename a photo</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <FormControl>
                                    <FormLabel>Old photo name</FormLabel>
                                    <Input type="text" defaultValue={props.name***REMOVED*** readOnly />
                                </FormControl>
                                <FormControl isInvalid={isError***REMOVED***>
                                    <FormLabel>New photo name</FormLabel>
                                    <Input type="text" placeholder="New photo name" onChange={handlePhotoRename***REMOVED*** />
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isError && (
                                        <FormErrorMessage>
                                            Photo name must not be empty.
                                        </FormErrorMessage>
                                    )***REMOVED***
                                </FormControl>
                            </ModalBody>
                            <ModalFooter>
                                <Button
                                    colorScheme="teal"
                                    mr={3***REMOVED***
                                    onClick={renameRequest***REMOVED***
                                    disabled={isError***REMOVED***
                                    isLoading={isLoading***REMOVED***
                                >
                                    Update
                                </Button>
                                <Button variant="ghost" mr={3***REMOVED*** onClick={handleResetNewName***REMOVED***>
                                    Cancel
                                </Button>
                            </ModalFooter>
                        </ModalContent>
                    </Modal>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default ImageItem