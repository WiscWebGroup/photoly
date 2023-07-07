import { Box, Button, ButtonGroup, FormControl, FormLabel, Input, Popover, PopoverArrow, PopoverCloseButton, PopoverContent, Select, Stack, useDisclosure } from "@chakra-ui/react"
import { useState, useRef, ChangeEvent } from "react";
import { AiOutlineStar, AiOutlineFlag } from "react-icons/ai";
import { BiPaperPlane } from "react-icons/bi";
import { FaUmbrellaBeach } from "react-icons/fa";
import { GiMountainCave } from "react-icons/gi";
import { GrFavorite } from "react-icons/gr";
import { HiOutlineAcademicCap } from "react-icons/hi";
import { TbAlbum } from "react-icons/tb";
import { ContextMenu } from "../ContextMenu"
import AlbumContextMenu from "../contextMenus/AlbumContextMenu"
import { useAppDispatch, useAppSelector } from "../../redux/hooks";
import { clearAlbum, setAlbum } from "../../redux/states/searchFilterSlice";
import { useDeleteAlbumMutation, useUpdateAlbumMutation } from "../../redux/api/albumSlice";

interface AlbumItemProps {
    id: number
    name: string
    coverId: number
    coverColor: string
}

const AlbumItem = ({ id, name, coverId, coverColor } : AlbumItemProps) => {
    const [newName, setNewName] = useState(name)
    const [newCoverId, setNewCoverId] = useState(coverId)
    const [newCoverColor, setNewCovereColor] = useState(coverColor)
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const inputRef = useRef<HTMLInputElement>(null)
    const [deleteAlbum, deleteStatus] = useDeleteAlbumMutation()
    const [updateAlbum, renameStatus] = useUpdateAlbumMutation()

    const { isOpen: isReOpen, onOpen: onReOpen, onClose: onReClose } = useDisclosure()
    const { isOpen: isIcOpen, onOpen: onIcOpen, onClose: onIcClose } = useDisclosure()

    const albumId = useAppSelector((state) => state.searchFilter.albumId)
    const dispatch = useAppDispatch()

    const handleReChange = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0)
        setNewName(value)
    }

    const handleReClose = () => {
        setNewName(name)
        onReClose()
    }

    const handleRename = () => {
        // setIsLoading(true)
        // updateRequest({id, name: newName, coverId, coverColor})
        // setIsLoading(false)
        updateAlbum({id, name: newName, coverId, coverColor})
        handleReClose()
    }

    const handleIcClose = () => {
        setNewCoverId(coverId)
        setNewCovereColor(coverColor)
        onIcClose()
    }

    const handleUpdateIcon = () => {
        if (coverId !== newCoverId || coverColor != newCoverColor) {
            // setIsLoading(true)
            // updateRequest({id, name, coverId: newCoverId, coverColor: newCoverColor})
            // setIsLoading(false)
            updateAlbum({id, name, coverId: newCoverId, coverColor: newCoverColor})
        }
        handleIcClose()
    }

    const handleAlbumClick = () => {
        // Want to de-select current album
        if (albumId !== -1 && id === albumId) {
            dispatch(clearAlbum())
        } else {
            dispatch(setAlbum(id))
        }
    }

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => 
                <AlbumContextMenu 
                    handleDelete={() => deleteAlbum(id)}
                    handleRename={onReOpen}
                    handleChangeIcon={onIcOpen}
                />}
        >
            {ref => (
                <Box ref={ref} w="100%" position="relative" pl={4} display="flex">
                    <Button 
                        variant="ghost" 
                        isActive={albumId === id}
                        w="100%" 
                        justifyContent="flex-start"
                        fontWeight="normal"
                        leftIcon={getCoverIcon(coverId, coverColor)}
                        // onClick={() => searchUpdateDispatch({type: "set_album", payload: id})}
                        onClick={handleAlbumClick}
                    >
                        {name}
                    </Button>

                    {/* rename popover */}
                    <Popover
                        isOpen={isReOpen}
                        initialFocusRef={inputRef}
                        onOpen={onReOpen}
                        onClose={handleReClose}
                        placement='auto'
                        closeOnBlur={false}
                    >
                        <PopoverContent p={5}>
                            <PopoverArrow />
                            <PopoverCloseButton />
                            <Stack spacing={4}>
                                <FormControl>
                                    <FormLabel>New album name</FormLabel>
                                    <Input size='sm' ref={inputRef} onChange={handleReChange} />
                                </FormControl>
                                <ButtonGroup display='flex' justifyContent='flex-end'>
                                    <Button size='sm' variant='outline' onClick={handleReClose}>
                                        Cancel
                                    </Button>
                                    <Button 
                                        size='sm' 
                                        isDisabled={isError} 
                                        isLoading={isLoading} 
                                        onClick={handleRename} 
                                        colorScheme='teal'
                                    >
                                        Update
                                    </Button>
                                </ButtonGroup>
                            </Stack>
                        </PopoverContent>
                    </Popover>

                    {/* icon popover */}
                    <Popover
                        isOpen={isIcOpen}
                        initialFocusRef={inputRef}
                        onOpen={onIcOpen}
                        onClose={handleIcClose}
                        placement='auto'
                        closeOnBlur={false}
                    >
                        <PopoverContent p={5}>
                            <PopoverArrow />
                            <PopoverCloseButton />
                            <Stack spacing={4}>
                                <FormControl>
                                    <FormLabel>Select Icon</FormLabel>
                                    <Select
                                        placeholder="Icon Format"
                                        mt={3}
                                        onChange={(e) => setNewCoverId(parseInt(e.target.value))}
                                    >
                                        <option value={0}>Default</option>
                                        <option value={1}>Star</option>
                                        <option value={2}>Favorite</option>
                                        <option value={3}>Beach</option>
                                        <option value={4}>Plane</option>
                                        <option value={5}>Flag</option>
                                        <option value={6}>Academic</option>
                                        <option value={7}>Mountain</option>
                                    </Select>
                                </FormControl>
                                <FormControl>
                                    <FormLabel>Select Icon Color</FormLabel>
                                    <Input
                                        variant="filled"
                                        type="color"
                                        value={coverColor}
                                        onChange={(e) => setNewCovereColor(e.target.value)}
                                        mt={3}
                                    />
                                </FormControl>
                                <ButtonGroup display='flex' justifyContent='flex-end'>
                                    <Button size='sm' variant='outline' onClick={handleIcClose}>
                                        Cancel
                                    </Button>
                                    <Button 
                                        size='sm' 
                                        isLoading={isLoading} 
                                        onClick={handleUpdateIcon} 
                                        colorScheme='teal'
                                    >
                                        Update
                                    </Button>
                                </ButtonGroup>
                            </Stack>
                        </PopoverContent>
                    </Popover>
                </Box>
            )}
        </ContextMenu>
    )
}

const getCoverIcon = (coverId: number | null, coverColor: string) => {
    if (coverId === null || coverId === 0) {
      return <TbAlbum color={coverColor} />;
    } else if (coverId === 1) {
      return <AiOutlineStar color={coverColor} />;
    } else if (coverId === 2) {
      return <GrFavorite color={coverColor} />;
    } else if (coverId === 3) {
      return <FaUmbrellaBeach color={coverColor} />;
    } else if (coverId === 4) {
      return <BiPaperPlane color={coverColor} />;
    } else if (coverId === 5) {
      return <AiOutlineFlag color={coverColor} />;
    } else if (coverId === 6) {
      return <HiOutlineAcademicCap color={coverColor} />;
    } else if (coverId === 7) {
      return <GiMountainCave color={coverColor} />;
    }
  };

export default AlbumItem