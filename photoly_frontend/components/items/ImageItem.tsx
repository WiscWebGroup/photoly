import {Box, Button, Center, FormControl, FormErrorMessage, FormLabel, Image, ImageProps, Input, Link, Modal, ModalBody, ModalCloseButton, ModalContent, ModalFooter, ModalHeader, ModalOverlay, Text, useDisclosure, useToast} from "@chakra-ui/react"
import {ContextMenu} from "../ContextMenu"
import ImageContextMenu from "../contextMenus/ImageContextMenu"
import ImageView from "../ImageView"
import {ChangeEvent, useEffect, useRef, useState} from "react";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";

interface ImageItemProps {
    pid: number
    name: string
    format: string
    uploaddate: string
    orgsrc: string
    folder_name: string
    refresh: () => void
}
interface TagLookup{
    photo_id: number
    tag_name: string
    tag_id: number
}
interface GalleryLookup{
    ga_id: number
    ga_name: string
    photo_id: number
}

const ImageItem = (props: Exclude<ImageProps & ImageItemProps, "onClick" | "borderRadius">) => {
    const {isOpen, onOpen, onClose} = useDisclosure()
    const [tags, setTags] = useState<string[]>([]);
    const [galleries, setGalleries] = useState<string[]>([]);
    const {get, post} = useApi()
    const token = useToken()
    const toast = useToast();

    const [newName, setNewName] = useState('')
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const {isOpen: isRenameOpen, onOpen: onRenameOpen, onClose: onRenameClose} = useDisclosure()

    const aRef = useRef<HTMLAnchorElement>(null)

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
        const getGalleries = async () => {
            await get(`/photo/getGalleryByPhoto?photoId=${props.pid}`, {
                headers: {"HRD-token": token}
            }).then((response) => {
                if (!!response && response.data && response.data.msgCode === 200) {
                    const temp: string[] = []
                    response.data.t.forEach((item: GalleryLookup) => {
                        temp.push(item.ga_name)
                    })
                    setGalleries(temp)
                }
            })
        }
        if (token){
            getTags();
            getGalleries();
        }
    }, [token])

    const deleteRequest = async () => {
        const response = await post("/photo/delete", null, {
            headers: { "HRD-token": token },
            params: {
              photoId: props.pid,
            },
        });

        if (!!response && response.data && response.data.msgCode === 200) {
            props.refresh()
        } else {
            toast({
                title: `Failed to delete the photo "${props.name}.${props.format}}"`,
                status: 'error',
                isClosable: true,
                position: 'top'
            })
        }
    }

    const renameRequest = async () => {
        setIsLoading(true)
        const response = await post("/photo/update", 
            {
                photoId: props.pid,
                photoName: newName,
            }, 
            {
                headers: { "HRD-token": token }
            }
        );

        if (!!response && response.data && response.data.msgCode === 200) {
            props.refresh()
        } else {
            toast({
                title: `Failed to rename the photo "${props.name}.${props.format}}"`,
                status: 'error',
                isClosable: true,
                position: 'top'
            })
        }
        setIsLoading(false);
        handleResetNewName();
    }

    const handleResetNewName = () => {
        setNewName('')
        setIsError(true)
        onRenameClose()
    }

    const handlePhotoRename = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value;
        setIsError(value.length === 0);
        setNewName(value);
    }

    const handleDownload = () => {
        aRef.current?.click();
    }

    return (
        <ContextMenu<HTMLDivElement>
            stopPropagation
            renderMenu={() => (
                <ImageContextMenu 
                    handleDelete={deleteRequest}
                    handleView={onOpen}
                    handleDownload={handleDownload}
                    handleRename={onRenameOpen}
                    handleMoveTo={() => {}}
                />
            )}
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
                    <Link ref={aRef} href={props.orgsrc} display="none" download />
                    <Center bg={"white"} w={"100%"}>
                        <Text fontWeight={"hairline"} whiteSpace={"nowrap"} overflow={"hidden"} textOverflow={"ellipsis"}>{props.name + "." + props.format}</Text>
                    </Center>
                    <ImageView isViewOpen={isOpen} onViewClose={onClose} path={props.folder_name} pname={props.name}
                               date={props.uploaddate} orgsrc={props.orgsrc} tags={tags} albums={galleries}/>
                    
                    {/* change name modal */}
                    <Modal isOpen={isRenameOpen} onClose={handleResetNewName}>
                        <ModalOverlay />
                        <ModalContent>
                            <ModalHeader>Rename a photo</ModalHeader>
                            <ModalCloseButton />
                            <ModalBody>
                                <FormControl>
                                    <FormLabel>Old photo name</FormLabel>
                                    <Input type="text" defaultValue={props.name} readOnly />
                                </FormControl>
                                <FormControl isInvalid={isError}>
                                    <FormLabel>New photo name</FormLabel>
                                    <Input type="text" placeholder="New photo name" onChange={handlePhotoRename} />
                                    {isError && (
                                        <FormErrorMessage>
                                            Photo name must not be empty.
                                        </FormErrorMessage>
                                    )}
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
                        </ModalContent>
                    </Modal>
                </Box>
            )}
        </ContextMenu>
    )
}

export default ImageItem