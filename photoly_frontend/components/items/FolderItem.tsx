import { Box, Button, FormControl, FormErrorMessage, FormLabel, Input, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast } from "@chakra-ui/react"
import { ChangeEvent, useState } from "react"
import { AiOutlineFolderOpen } from "react-icons/ai"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"
import { ContextMenu } from "../ContextMenu"
import FolderContextMenu from "../contextMenus/FolderContextMenu"
import { useOpenFolder } from "../contexts/SearchContext"

interface FolderItemProps {
    id: number
    name: string
    parentId: number
}

const FolderItem = ({id, name, parentId} : FolderItemProps) => {
    const openFolder = useOpenFolder()
    const [newName, setNewName] = useState("")
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const { isOpen: isRenameOpen, onOpen: onRenameOpen, onClose: onRenameClose } = useDisclosure()

    const token = useToken()
    const { post } = useApi()
    const toast = useToast()

    const deleteRequest = async () => {
        const response = await post("/namespace/delete", null, {
            headers: { "HRD-token": token },
            params: {
                nsId: id
            }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(parentId)
        } else {
            toast({title: `Failed to delete the folder "${name}"`, status: "error", isClosable: true, position: "top"})
        }
    }

    const handleFolderRename = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0 || value === "/")
        setNewName(value)
    }

    const handleResetNewName = () => {
        setNewName("")
        setIsError(true)
        onRenameClose()
    }

    const renameRequest = async () => {
        setIsLoading(true)
        const response = await post("/namespace/updateName", {
            nsId: id,
            nsName: newName
        }, {
            headers: { "HRD-token": token }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(parentId)
        } else {
            toast({title: `Failed to rename the folder "${name}" to "${newName}"`, status: "error", isClosable: true, position: "top"})
        }
        setIsLoading(false)
        handleResetNewName()
    }

    // TODO: handle moveto

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => <FolderContextMenu 
                handleDelete={deleteRequest}
                handleRename={onRenameOpen}
                handleMoveTo={() => {}}
            />}
        >
            {ref => (
                <Box ref={ref} w={64} overflow='hidden' position='relative' m={2} >
                    <Button 
                        leftIcon={<AiOutlineFolderOpen />} 
                        colorScheme='gray' 
                        variant='solid' 
                        w={"100%"}
                        onClick={() => openFolder(id)}
                    >
                        {name}
                    </Button>

                    <Modal isOpen={isRenameOpen} onClose={handleResetNewName}>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Rename a folder</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <FormControl>
                                    <FormLabel>Old folder name</FormLabel>
                                    <Input type="text" defaultValue={name} readOnly />
                                </FormControl>
                                <FormControl isInvalid={isError}>
                                    <FormLabel>New folder name</FormLabel>
                                    <Input 
                                        type="text" 
                                        placeholder="New folder name" 
                                        onChange={handleFolderRename}
                                    />
                                    {isError &&
                                        <FormErrorMessage>Folder name must not be empty or "/"</FormErrorMessage>
                                    }
                                </FormControl>
                            </ModalBody>
                            <ModalFooter>
                                <Button 
                                    colorScheme="teal" 
                                    mr={3} 
                                    onClick={renameRequest}
                                    disabled={isError}
                                    isLoading={isLoading}
                                >
                                    Update
                                </Button>
                                <Button variant="ghost" mr={3} onClick={handleResetNewName}>
                                    Cancel
                                </Button>
                            </ModalFooter>
                        </ ModalContent>
                    </Modal>
                </Box>
            )}
        </ContextMenu>
    )
}

export default FolderItem