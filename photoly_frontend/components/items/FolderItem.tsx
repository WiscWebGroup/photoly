import { Box, Button, FormControl, FormErrorMessage, FormLabel, Input, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast ***REMOVED*** from "@chakra-ui/react"
import { ChangeEvent, useState ***REMOVED*** from "react"
import { AiOutlineFolderOpen ***REMOVED*** from "react-icons/ai"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"
import { ContextMenu ***REMOVED*** from "../ContextMenu"
import FolderContextMenu from "../contextMenus/FolderContextMenu"
import { useOpenFolder ***REMOVED*** from "../contexts/SearchContext"

interface FolderItemProps {
    id: number
    name: string
    parentId: number
***REMOVED***

const FolderItem = ({id, name, parentId***REMOVED*** : FolderItemProps) => {
    const openFolder = useOpenFolder()
    const [newName, setNewName] = useState("")
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const { isOpen: isRenameOpen, onOpen: onRenameOpen, onClose: onRenameClose ***REMOVED*** = useDisclosure()

    const token = useToken()
    const { post ***REMOVED*** = useApi()
    const toast = useToast()

    const deleteRequest = async () => {
        const response = await post("/namespace/delete", null, {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
                nsId: id
    ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(parentId)
***REMOVED*** else {
            toast({title: `Failed to delete the folder "${name***REMOVED***"`, status: "error", isClosable: true, position: "top"***REMOVED***)
***REMOVED***
***REMOVED***

    const handleFolderRename = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0 || value === "/")
        setNewName(value)
***REMOVED***

    const handleResetNewName = () => {
        setNewName("")
        setIsError(true)
        onRenameClose()
***REMOVED***

    const renameRequest = async () => {
        setIsLoading(true)
        const response = await post("/namespace/updateName", {
            nsId: id,
            nsName: newName
***REMOVED***, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            openFolder(parentId)
***REMOVED*** else {
            toast({title: `Failed to rename the folder "${name***REMOVED***" to "${newName***REMOVED***"`, status: "error", isClosable: true, position: "top"***REMOVED***)
***REMOVED***
        setIsLoading(false)
        handleResetNewName()
***REMOVED***

    // TODO: handle moveto

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => <FolderContextMenu 
                handleDelete={deleteRequest***REMOVED***
                handleRename={onRenameOpen***REMOVED***
                handleMoveTo={() => {***REMOVED******REMOVED***
            />***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w={64***REMOVED*** overflow='hidden' position='relative' m={2***REMOVED*** >
                    <Button 
                        leftIcon={<AiOutlineFolderOpen />***REMOVED*** 
                        colorScheme='gray' 
                        variant='solid' 
                        w={"100%"***REMOVED***
                        onClick={() => openFolder(id)***REMOVED***
                    >
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***name***REMOVED***
                    </Button>

                    <Modal isOpen={isRenameOpen***REMOVED*** onClose={handleResetNewName***REMOVED***>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Rename a folder</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <FormControl>
                                    <FormLabel>Old folder name</FormLabel>
                                    <Input type="text" defaultValue={name***REMOVED*** readOnly />
                                </FormControl>
                                <FormControl isInvalid={isError***REMOVED***>
                                    <FormLabel>New folder name</FormLabel>
                                    <Input 
                                        type="text" 
                                        placeholder="New folder name" 
                                        onChange={handleFolderRename***REMOVED***
                                    />
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isError &&
                                        <FormErrorMessage>Folder name must not be empty or "/"</FormErrorMessage>
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***
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
                        </ ModalContent>
                    </Modal>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

export default FolderItem