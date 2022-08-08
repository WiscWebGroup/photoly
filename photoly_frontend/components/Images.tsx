import {Box, Button, Flex, Input, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast} from "@chakra-ui/react";
import React, { ChangeEvent, useRef, useState } from "react";
import useApi from "../hooks/useApi";
import useToken from "../hooks/useToken";
import { ContextMenu } from "./ContextMenu";
import PanelContextMenu from "./contextMenus/PanelContextMenu";
import { useOpenFolder, useSearchData } from "./contexts/SearchContext";
import FolderItem from "./items/FolderItem";

import ImageItem from "./items/ImageItem";

const Images: React.FC = () => {
    const { isOpen: isUpOpen, onOpen: onUpOpen, onClose: onUpClose } = useDisclosure()
    const { isOpen: isCrOpen, onOpen: onCrOpen, onClose: onCrClose } = useDisclosure()

    const [files, setFiles] = useState<Blob[] | string[]>([])
    const fileRef = useRef<HTMLInputElement>(null)
    const [isLoading, setIsLoading] = useState(false)

    const [folderName, setFolderName] = useState("")
    const [isError, setIsError] = useState(true)

    const token = useToken()
    const { post } = useApi()
    const toast = useToast()
    const openFolder = useOpenFolder()
    const searchData = useSearchData()

    const handleUploadFile = (e: ChangeEvent<HTMLInputElement>) => {
        const fs: FileList | null = e.target.files
        if (fs !== null) {
            let fileArray: File[] = []
            for (let i = 0; i < fs.length; i++) {
                let f = fs.item(i)
                if (f !== null && f.size <= 600000000) {
                    fileArray.push(f)
                }
            }
            setFiles(fileArray)
        }
    }

    const handleRestFile = () => {
        if (fileRef.current) fileRef.current.value = ""
        onUpClose()
    }

    const handleUpload = async () => {
        setIsLoading(true)
        const formData = new FormData()
        for (const file of files)
            formData.append("files", file)
        const part = JSON.stringify({
            nsId: searchData.current.id,
            visibility: 1
        })
        const photosStr = Array(files.length).fill(part)
        const response = await post("/photo/inserts", formData, {
            headers: { "HRD-token": token },
            params: { photosStr: "[" + photosStr + "]" }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(searchData.current.id)
        } else {
            toast({title: "Failed to upload the photo", status: "error", isClosable: true, position: "top"})
        }

        setIsLoading(false)
        handleRestFile()
    }

    const handleNewFolderName = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0 || value === "/")
        setFolderName(value)
    }

    const handleResetCreation = () => {
        setFolderName("")
        setIsError(true)
        onCrClose()
    }

    const handleCreate = async () => {
        setIsLoading(true)
        const response = await post("/namespace/insert", {
            nsParentId: searchData.current.id,
            nsName: folderName
        }, {
            headers: { "HRD-token": token }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(searchData.current.id)
        } else {
            toast({title: `Failed to create a new folder named ${folderName}`, status: "error", isClosable: true, position: "top"})
        }
        setIsLoading(false)
        handleResetCreation()
    }

    return (
        <ContextMenu<HTMLDivElement> 
            renderMenu={() => 
                <PanelContextMenu 
                    triggerUpload={onUpOpen}
                    triggerCreate={onCrOpen}
                />}
        >
            {ref => (
                <Box ref={ref} w='100%' h='100%'>
                    <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Folders</Text>
                    <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                        {searchData.folders.map((folder, index) => {
                            return <FolderItem 
                                        key={index} 
                                        id={folder.id} 
                                        name={folder.name} 
                                        parentId={searchData.current.id} 
                                    />
                        })}
                    </Flex>
                    <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Images</Text>
                    <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                        {searchData.photos.map((photo, ind) => 
                            <ImageItem 
                                key={ind} 
                                src={`/photo/render/${token}?photoId=${photo.id}`} 
                                pid={photo.id}
                                name={photo.name}
                                format={photo.format}
                            />
                        )}
                    </Flex>

                    {/* upload modal */}
                    <Modal isOpen={isUpOpen} onClose={handleRestFile}>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Choose a photo</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <Input 
                                    multiple
                                    ref={fileRef} 
                                    type="file" 
                                    accept="image/jpeg, image/png" 
                                    onChange={handleUploadFile}
                                />
                            </ModalBody>
                            <ModalFooter>
                                <Button 
                                    colorScheme="teal" 
                                    mr={3} 
                                    onClick={handleUpload}
                                    isLoading={isLoading}
                                >
                                    Upload
                                </Button>
                                <Button variant="ghost" mr={3} onClick={handleRestFile}>
                                    Cancel
                                </Button>
                            </ModalFooter>
                        </ModalContent>
                    </Modal>

                    {/* create modal */}
                    <Modal isOpen={isCrOpen} onClose={handleResetCreation}>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Create a folder</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <Input 
                                    type="text" 
                                    placeholder="New folder name" 
                                    onChange={handleNewFolderName}
                                />
                                {isError && (
                                    <Text fontSize="sm" color="red">
                                        Folder name must not be empty or "/"
                                    </Text>
                                )}
                            </ModalBody>
                            <ModalFooter>
                                <Button 
                                    colorScheme="teal" 
                                    mr={3} 
                                    onClick={handleCreate}
                                    disabled={isError}
                                    isLoading={isLoading}
                                >
                                    Create
                                </Button>
                                <Button variant="ghost" mr={3} onClick={handleResetCreation}>
                                    Cancel
                                </Button>
                            </ModalFooter>
                        </ModalContent>
                    </Modal>
                </Box>
            )}
        </ContextMenu>
    )
}
export default Images