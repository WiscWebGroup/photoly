import { AddIcon } from "@chakra-ui/icons"
import { Box, HStack, Icon, IconButton, Text } from "@chakra-ui/react"
import { ReactNode } from "react"
import { IconType } from "react-icons"

interface TagHeaderProps {
    headerIcon?: IconType
    iconColor?: string
    onAdd?: () => void
    children?: ReactNode
}

const TagHeader = ({headerIcon, iconColor, onAdd, children}: TagHeaderProps) => {
    return (
        <Box style={{width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <HStack>
                <Icon as={headerIcon} color={iconColor} />
                <Text>{children}</Text>
            </HStack>
            <IconButton variant="unstyled" size="xs" aria-label="Add tag" onClick={onAdd} icon={<AddIcon />} />
        </Box>
    )
}

export default TagHeader