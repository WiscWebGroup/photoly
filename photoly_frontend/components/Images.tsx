import {Box, Button, Flex, Input, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast***REMOVED*** from "@chakra-ui/react";
import React, { ChangeEvent, useRef, useState ***REMOVED*** from "react";
import useApi from "../hooks/useApi";
import useToken from "../hooks/useToken";
import { ContextMenu ***REMOVED*** from "./ContextMenu";
import PanelContextMenu from "./contextMenus/PanelContextMenu";
import { useOpenFolder, useSearchData ***REMOVED*** from "./contexts/SearchContext";
import FolderItem from "./items/FolderItem";

import ImageItem from "./items/ImageItem";

const Images: React.FC = () => {
    const { isOpen: isUpOpen, onOpen: onUpOpen, onClose: onUpClose ***REMOVED*** = useDisclosure()
    const { isOpen: isCrOpen, onOpen: onCrOpen, onClose: onCrClose ***REMOVED*** = useDisclosure()

    const [file, setFile] = useState<Blob | string>("")
    const fileRef = useRef<HTMLInputElement>(null)
    const [isOversize, setIsOversize] = useState(false)
    const [isDisabled, setIsDisabled] = useState(true)
    const [isLoading, setIsLoading] = useState(false)

    const [folderName, setFolderName] = useState("")
    const [isError, setIsError] = useState(true)

    const token = useToken()
    const { post ***REMOVED*** = useApi()
    const toast = useToast()
    const openFolder = useOpenFolder()
    const searchData = useSearchData()

    const handleUploadFile = (e: ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files[0]) {
            const f = e.target.files[0]
            if (f.size <= 600000000) {
                setFile(f)
                setIsOversize(false)
                setIsDisabled(false)
                return
    ***REMOVED***
            setIsOversize(true)
***REMOVED***
        setIsDisabled(true)
***REMOVED***

    const handleRestFile = () => {
        if (fileRef.current) fileRef.current.value = ""
        setIsOversize(false)
        setIsDisabled(true)
        onUpClose()
***REMOVED***

    const handleUpload = async () => {
        setIsLoading(true)
        const formData = new FormData()
        formData.append("file", file)
        const response = await post("/photo/insert", formData, {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
                photo: {
                    nsId: searchData.current.id,
                    visibility: 1
   ***REMOVED*****REMOVED***
    ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(searchData.current.id)
***REMOVED*** else {
            toast({title: "Failed to upload the photo", status: "error", isClosable: true, position: "top"***REMOVED***)
***REMOVED***

        setIsLoading(false)
        handleRestFile()
***REMOVED***

    const handleNewFolderName = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0 || value === "/")
        setFolderName(value)
***REMOVED***

    const handleResetCreation = () => {
        setFolderName("")
        setIsError(true)
        onCrClose()
***REMOVED***

    const handleCreate = async () => {
        setIsLoading(true)
        const response = await post("/namespace/insert", {
            nsParentId: searchData.current.id,
            nsName: folderName
***REMOVED***, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(searchData.current.id)
***REMOVED*** else {
            toast({title: `Failed to create a new folder named ${folderName***REMOVED***`, status: "error", isClosable: true, position: "top"***REMOVED***)
***REMOVED***
        setIsLoading(false)
        handleResetCreation()
***REMOVED***

    return (
        <ContextMenu<HTMLDivElement> 
            renderMenu={() => 
                <PanelContextMenu 
                    triggerUpload={onUpOpen***REMOVED***
                    triggerCreate={onCrOpen***REMOVED***
                />***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w='100%' h='100%'>
                    <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Folders</Text>
                    <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***searchData.folders.map((folder, index) => {
                            return <FolderItem 
                                        key={index***REMOVED*** 
                                        id={folder.id***REMOVED*** 
                                        name={folder.name***REMOVED*** 
                                        parentId={searchData.current.id***REMOVED*** 
                                    />
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***)***REMOVED***
                    </Flex>
                    <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Images</Text>
                    <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***searchData.photos.map((photo, ind) => 
                            <ImageItem 
                                key={ind***REMOVED*** 
                                src={`/photo/render/${token***REMOVED***?photoId=${photo.id***REMOVED***`***REMOVED*** 
                                pid={photo.id***REMOVED***
                                name={photo.name***REMOVED***
                                format={photo.format***REMOVED***
                            />
                        )***REMOVED***
                    </Flex>

 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***/* upload modal***REMOVED******REMOVED***
                    <Modal isOpen={isUpOpen***REMOVED*** onClose={handleRestFile***REMOVED***>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Choose a photo</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <Input 
                                    ref={fileRef***REMOVED*** 
                                    type="file" 
                                    accept="image/jpeg, image/png" 
                                    onChange={handleUploadFile***REMOVED***
                                />
             ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isOversize && (
                                    <Text fontSize="sm" color="red">
                                        File size should not exceed 200 KB.
                                    </Text>
                                )***REMOVED***
                            </ModalBody>
                            <ModalFooter>
                                <Button 
                                    colorScheme="teal" 
                                    mr={3***REMOVED*** 
                                    onClick={handleUpload***REMOVED***
                                    disabled={isDisabled***REMOVED***
                                    isLoading={isLoading***REMOVED***
                                >
                                    Upload
                                </Button>
                                <Button variant="ghost" mr={3***REMOVED*** onClick={handleRestFile***REMOVED***>
                                    Cancel
                                </Button>
                            </ModalFooter>
                        </ModalContent>
                    </Modal>

 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***/* create modal***REMOVED******REMOVED***
                    <Modal isOpen={isCrOpen***REMOVED*** onClose={handleResetCreation***REMOVED***>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Create a folder</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <Input 
                                    type="text" 
                                    placeholder="New folder name" 
                                    onChange={handleNewFolderName***REMOVED***
                                />
             ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isError && (
                                    <Text fontSize="sm" color="red">
                                        Folder name must not be empty or "/"
                                    </Text>
                                )***REMOVED***
                            </ModalBody>
                            <ModalFooter>
                                <Button 
                                    colorScheme="teal" 
                                    mr={3***REMOVED*** 
                                    onClick={handleCreate***REMOVED***
                                    disabled={isError***REMOVED***
                                    isLoading={isLoading***REMOVED***
                                >
                                    Create
                                </Button>
                                <Button variant="ghost" mr={3***REMOVED*** onClick={handleResetCreation***REMOVED***>
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
export default Images