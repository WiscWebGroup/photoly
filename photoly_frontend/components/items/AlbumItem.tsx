import { Box, Button, ButtonGroup, FormControl, FormLabel, Input, Popover, PopoverArrow, PopoverCloseButton, PopoverContent, Select, Stack, useDisclosure ***REMOVED*** from "@chakra-ui/react"
import { useState, useRef, ChangeEvent ***REMOVED*** from "react";
import { AiOutlineStar, AiOutlineFlag ***REMOVED*** from "react-icons/ai";
import { BiPaperPlane ***REMOVED*** from "react-icons/bi";
import { FaUmbrellaBeach ***REMOVED*** from "react-icons/fa";
import { GiMountainCave ***REMOVED*** from "react-icons/gi";
import { GrFavorite ***REMOVED*** from "react-icons/gr";
import { HiOutlineAcademicCap ***REMOVED*** from "react-icons/hi";
import { TbAlbum ***REMOVED*** from "react-icons/tb";
import { ContextMenu ***REMOVED*** from "../ContextMenu"
import AlbumContextMenu from "../contextMenus/AlbumContextMenu"
import { useAlbumListUpdate ***REMOVED*** from "../contexts/AlbumContext";
import { useAppDispatch, useAppSelector ***REMOVED*** from "../../redux/hooks";
import { clearAlbum, setAlbum ***REMOVED*** from "../../redux/states/searchFilterSlice";
import {useDeleteAlbumMutation***REMOVED*** from "../../redux/api/albumSlice";

interface AlbumItemProps {
    id: number
    name: string
    coverId: number
    coverColor: string
***REMOVED***

const AlbumItem = ({ id, name, coverId, coverColor ***REMOVED*** : AlbumItemProps) => {
    const [newName, setNewName] = useState(name)
    const [newCoverId, setNewCoverId] = useState(coverId)
    const [newCoverColor, setNewCovereColor] = useState(coverColor)
    const [isError, setIsError] = useState(true)
    const [isLoading, setIsLoading] = useState(false)
    const inputRef = useRef<HTMLInputElement>(null)
    const [deleteAlbum, _] = useDeleteAlbumMutation()

    const { isOpen: isReOpen, onOpen: onReOpen, onClose: onReClose ***REMOVED*** = useDisclosure()
    const { isOpen: isIcOpen, onOpen: onIcOpen, onClose: onIcClose ***REMOVED*** = useDisclosure()

    const { updateRequest, deleteRequest ***REMOVED*** = useAlbumListUpdate()

    const albumId = useAppSelector((state) => state.searchFilter.albumId)
    const dispatch = useAppDispatch()

    const handleReChange = (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.currentTarget.value
        setIsError(value.length === 0)
        setNewName(value)
***REMOVED***

    const handleReClose = () => {
        setNewName(name)
        onReClose()
***REMOVED***

    const handleRename = () => {
        setIsLoading(true)
        updateRequest({id, name: newName, coverId, coverColor***REMOVED***)
        setIsLoading(false)
        handleReClose()
***REMOVED***

    const handleIcClose = () => {
        setNewCoverId(coverId)
        setNewCovereColor(coverColor)
        onIcClose()
***REMOVED***

    const handleUpdateIcon = () => {
        if (coverId !== newCoverId || coverColor != newCoverColor) {
            setIsLoading(true)
            updateRequest({id, name, coverId: newCoverId, coverColor: newCoverColor***REMOVED***)
            setIsLoading(false)
***REMOVED***
        handleIcClose()
***REMOVED***

    const handleAlbumClick = () => {
        // Want to de-select current album
        if (albumId !== -1 && id === albumId) {
            dispatch(clearAlbum())
***REMOVED*** else {
            dispatch(setAlbum(id))
***REMOVED***
***REMOVED***

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => 
                <AlbumContextMenu 
                    handleDelete={() => deleteAlbum(id)***REMOVED***
                    handleRename={onReOpen***REMOVED***
                    handleChangeIcon={onIcOpen***REMOVED***
                />***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w="100%" position="relative" pl={4***REMOVED*** display="flex">
                    <Button 
                        variant="ghost" 
                        isActive={albumId === id***REMOVED***
                        w="100%" 
                        justifyContent="flex-start"
                        fontWeight="normal"
                        leftIcon={getCoverIcon(coverId, coverColor)***REMOVED***
                        // onClick={() => searchUpdateDispatch({type: "set_album", payload: id***REMOVED***)***REMOVED***
                        onClick={handleAlbumClick***REMOVED***
                    >
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***name***REMOVED***
                    </Button>

 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***/* rename popover***REMOVED******REMOVED***
                    <Popover
                        isOpen={isReOpen***REMOVED***
                        initialFocusRef={inputRef***REMOVED***
                        onOpen={onReOpen***REMOVED***
                        onClose={handleReClose***REMOVED***
                        placement='auto'
                        closeOnBlur={false***REMOVED***
                    >
                        <PopoverContent p={5***REMOVED***>
                            <PopoverArrow />
                            <PopoverCloseButton />
                            <Stack spacing={4***REMOVED***>
                                <FormControl>
                                    <FormLabel>New album name</FormLabel>
                                    <Input size='sm' ref={inputRef***REMOVED*** onChange={handleReChange***REMOVED*** />
                                </FormControl>
                                <ButtonGroup display='flex' justifyContent='flex-end'>
                                    <Button size='sm' variant='outline' onClick={handleReClose***REMOVED***>
                                        Cancel
                                    </Button>
                                    <Button 
                                        size='sm' 
                                        isDisabled={isError***REMOVED*** 
                                        isLoading={isLoading***REMOVED*** 
                                        onClick={handleRename***REMOVED*** 
                                        colorScheme='teal'
                                    >
                                        Update
                                    </Button>
                                </ButtonGroup>
                            </Stack>
                        </PopoverContent>
                    </Popover>

 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***/* icon popover***REMOVED******REMOVED***
                    <Popover
                        isOpen={isIcOpen***REMOVED***
                        initialFocusRef={inputRef***REMOVED***
                        onOpen={onIcOpen***REMOVED***
                        onClose={handleIcClose***REMOVED***
                        placement='auto'
                        closeOnBlur={false***REMOVED***
                    >
                        <PopoverContent p={5***REMOVED***>
                            <PopoverArrow />
                            <PopoverCloseButton />
                            <Stack spacing={4***REMOVED***>
                                <FormControl>
                                    <FormLabel>Select Icon</FormLabel>
                                    <Select
                                        placeholder="Icon Format"
                                        mt={3***REMOVED***
                                        onChange={(e) => setNewCoverId(parseInt(e.target.value))***REMOVED***
                                    >
                                        <option value={0***REMOVED***>Default</option>
                                        <option value={1***REMOVED***>Star</option>
                                        <option value={2***REMOVED***>Favorite</option>
                                        <option value={3***REMOVED***>Beach</option>
                                        <option value={4***REMOVED***>Plane</option>
                                        <option value={5***REMOVED***>Flag</option>
                                        <option value={6***REMOVED***>Academic</option>
                                        <option value={7***REMOVED***>Mountain</option>
                                    </Select>
                                </FormControl>
                                <FormControl>
                                    <FormLabel>Select Icon Color</FormLabel>
                                    <Input
                                        variant="filled"
                                        type="color"
                                        value={coverColor***REMOVED***
                                        onChange={(e) => setNewCovereColor(e.target.value)***REMOVED***
                                        mt={3***REMOVED***
                                    />
                                </FormControl>
                                <ButtonGroup display='flex' justifyContent='flex-end'>
                                    <Button size='sm' variant='outline' onClick={handleIcClose***REMOVED***>
                                        Cancel
                                    </Button>
                                    <Button 
                                        size='sm' 
                                        isLoading={isLoading***REMOVED*** 
                                        onClick={handleUpdateIcon***REMOVED*** 
                                        colorScheme='teal'
                                    >
                                        Update
                                    </Button>
                                </ButtonGroup>
                            </Stack>
                        </PopoverContent>
                    </Popover>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***

const getCoverIcon = (coverId: number | null, coverColor: string) => {
    if (coverId === null || coverId === 0) {
      return <TbAlbum color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 1) {
      return <AiOutlineStar color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 2) {
      return <GrFavorite color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 3) {
      return <FaUmbrellaBeach color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 4) {
      return <BiPaperPlane color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 5) {
      return <AiOutlineFlag color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 6) {
      return <HiOutlineAcademicCap color={coverColor***REMOVED*** />;
***REMOVED*** else if (coverId === 7) {
      return <GiMountainCave color={coverColor***REMOVED*** />;
***REMOVED***
  ***REMOVED***;

export default AlbumItem