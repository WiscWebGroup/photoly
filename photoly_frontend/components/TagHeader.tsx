import { AddIcon } from "@chakra-ui/icons"
import { Box, Button, ButtonGroup, FormControl, FormLabel, HStack, Icon, IconButton, Input, Popover, PopoverArrow, PopoverCloseButton, PopoverContent, PopoverTrigger, Stack, Text, useDisclosure } from "@chakra-ui/react"
import { ChangeEvent, ReactNode, useRef, useState } from "react"
import { IconType } from "react-icons"
import { useTagListUpdate } from "./contexts/TagContext"

interface TagHeaderProps {
    headerIcon?: IconType
    iconColor?: string
    children?: ReactNode
}

const TagHeader = ({headerIcon, iconColor, children}: TagHeaderProps) => {
    const inputRef = useRef(null)
    const [tag, setTag] = useState("")
    const { insertRequest } = useTagListUpdate()
    const [isDisabled, setIsDisabled] = useState(true)  // control submit button
    const { isOpen, onOpen, onClose } = useDisclosure()  // control popover

    const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const str = e.currentTarget.value
        setTag(str);
        setIsDisabled(str.length === 0)
    }

    const handleClose = () => {
        setTag("")
        onClose()
    }

    const handleInsert = () => {
        insertRequest(tag)
        handleClose()
    }

    return (
        <Box style={{width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <HStack>
                <Icon as={headerIcon} color={iconColor} />
                <Text>{children}</Text>
            </HStack>
            <Popover
                isOpen={isOpen}
                initialFocusRef={inputRef}
                onOpen={onOpen}
                onClose={handleClose}
                placement='auto'
                closeOnBlur={false}
            >
                <PopoverTrigger>
                    <IconButton variant="unstyled" size="xs" aria-label="Add tag" icon={<AddIcon />} />
                </PopoverTrigger>
                <PopoverContent p={5}>
                    <PopoverArrow />
                    <PopoverCloseButton />
                    <Stack spacing={4}>
                        <FormControl>
                            <FormLabel>New Tag</FormLabel>
                            <Input size='sm' ref={inputRef} onChange={handleChange} />
                        </FormControl>
                        <ButtonGroup display='flex' justifyContent='flex-end'>
                            <Button size='sm' variant='outline' onClick={handleClose}>
                                Cancel
                            </Button>
                            <Button size='sm' isDisabled={isDisabled} onClick={handleInsert} colorScheme='teal'>
                                Save
                            </Button>
                        </ButtonGroup>
                    </Stack>
                </PopoverContent>
            </Popover>
        </Box>
    )
}

export default TagHeader